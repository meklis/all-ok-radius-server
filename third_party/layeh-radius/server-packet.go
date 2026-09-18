package radius

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
)

type packetResponseWriter struct {
	// listener that received the packet
	conn net.PacketConn
	addr net.Addr
}

func (r *packetResponseWriter) Write(packet *Packet) error {
	encoded, err := packet.Encode()
	if err != nil {
		return err
	}
	if _, err := r.conn.WriteTo(encoded, r.addr); err != nil {
		return err
	}
	return nil
}

// PacketServer listens for RADIUS requests on a packet-based protocols (e.g.
// UDP).
type PacketServer struct {
	// The address on which the server listens. Defaults to :1812.
	Addr string

	// The network on which the server listens. Defaults to udp.
	Network string

	// The source from which the secret is obtained for parsing and validating
	// the request.
	SecretSource SecretSource

	// Handler which is called to process the request.
	Handler Handler

	// Skip incoming packet authenticity validation.
	// This should only be set to true for debugging purposes.
	InsecureSkipVerify bool

	// NumReaders is the number of goroutines concurrently calling
	// conn.ReadFrom on the listening socket. Defaults to 1 (the historical
	// behaviour of this package).
	//
	// A single reader can become the throughput ceiling for the whole
	// server under load: incoming datagrams queue up in the kernel socket
	// buffer and get silently dropped once it overflows, no matter how
	// much spare CPU the rest of the process has, because only one
	// goroutine is ever pulling packets off the wire. Multiple goroutines
	// can safely call ReadFrom concurrently on the same net.PacketConn -
	// the kernel/runtime hands each waiting reader the next datagram, so
	// this removes that single-goroutine ceiling. Everything after the
	// read (parsing, dedup, the actual Handler call) was already
	// dispatched to its own goroutine and is unaffected.
	NumReaders int

	// ReadBufferSize, if non-zero, sets the socket's receive buffer size
	// (SO_RCVBUF) via net.UDPConn.SetReadBuffer before serving. Only
	// applies when ListenAndServe creates the connection and the
	// underlying conn is a *net.UDPConn. The kernel silently clamps this
	// to net.core.rmem_max, so requesting more than that is harmless.
	ReadBufferSize int

	shutdownRequested int32

	mu          sync.Mutex
	ctx         context.Context
	ctxDone     context.CancelFunc
	listeners   map[net.PacketConn]uint
	lastActive  chan struct{} // closed when the last active item finishes
	activeCount int32
}

func (s *PacketServer) initLocked() {
	if s.ctx == nil {
		s.ctx, s.ctxDone = context.WithCancel(context.Background())
		s.listeners = make(map[net.PacketConn]uint)
		s.lastActive = make(chan struct{})
	}
}

func (s *PacketServer) activeAdd() {
	atomic.AddInt32(&s.activeCount, 1)
}

func (s *PacketServer) activeDone() {
	if atomic.AddInt32(&s.activeCount, -1) == -1 {
		close(s.lastActive)
	}
}

// TODO: logger on PacketServer

// Serve accepts incoming connections on conn.
func (s *PacketServer) Serve(conn net.PacketConn) error {
	if s.Handler == nil {
		return errors.New("radius: nil Handler")
	}
	if s.SecretSource == nil {
		return errors.New("radius: nil SecretSource")
	}

	s.mu.Lock()
	s.initLocked()
	if atomic.LoadInt32(&s.shutdownRequested) == 1 {
		s.mu.Unlock()
		return ErrServerShutdown
	}

	s.listeners[conn]++
	s.mu.Unlock()

	type requestKey struct {
		IP         string
		Identifier byte
	}

	var (
		requestsLock sync.Mutex
		requests     = map[requestKey]struct{}{}
	)

	s.activeAdd()
	defer func() {
		s.mu.Lock()
		s.listeners[conn]--
		if s.listeners[conn] == 0 {
			delete(s.listeners, conn)
		}
		s.mu.Unlock()
		s.activeDone()
	}()

	// readLoop is run by one or more goroutines below, each with its own
	// packet buffer (buff must not be shared across concurrent ReadFrom
	// calls). Everything past the read itself was already dispatched to
	// its own goroutine per packet, so this only parallelizes pulling
	// datagrams off the socket.
	readLoop := func() error {
		var buff [MaxPacketLength]byte
		for {
			n, remoteAddr, err := conn.ReadFrom(buff[:])
			if err != nil {
				if atomic.LoadInt32(&s.shutdownRequested) == 1 {
					return ErrServerShutdown
				}

				if ne, ok := err.(net.Error); ok && !ne.Temporary() {
					return err
				}
				continue
			}

			s.activeAdd()
			go func(buff []byte, remoteAddr net.Addr) {
				defer s.activeDone()

				secret, err := s.SecretSource.RADIUSSecret(s.ctx, remoteAddr)
				if err != nil {
					return
				}
				if len(secret) == 0 {
					return
				}

				if !s.InsecureSkipVerify && !IsAuthenticRequest(buff, secret) {
					return
				}

				packet, err := Parse(buff, secret)
				if err != nil {
					return
				}

				key := requestKey{
					IP:         remoteAddr.String(),
					Identifier: packet.Identifier,
				}
				requestsLock.Lock()
				if _, ok := requests[key]; ok {
					requestsLock.Unlock()
					return
				}
				requests[key] = struct{}{}
				requestsLock.Unlock()

				response := packetResponseWriter{
					conn: conn,
					addr: remoteAddr,
				}

				defer func() {
					requestsLock.Lock()
					delete(requests, key)
					requestsLock.Unlock()
				}()

				request := Request{
					LocalAddr:  conn.LocalAddr(),
					RemoteAddr: remoteAddr,
					Packet:     packet,
					ctx:        s.ctx,
				}

				s.Handler.ServeRADIUS(&response, &request)
			}(append([]byte(nil), buff[:n]...), remoteAddr)
		}
	}

	numReaders := s.NumReaders
	if numReaders <= 1 {
		return readLoop()
	}

	errs := make(chan error, numReaders)
	for i := 0; i < numReaders; i++ {
		s.activeAdd()
		go func() {
			defer s.activeDone()
			errs <- readLoop()
		}()
	}
	return <-errs
}

// ListenAndServe starts a RADIUS server on the address given in s.
func (s *PacketServer) ListenAndServe() error {
	if s.Handler == nil {
		return errors.New("radius: nil Handler")
	}
	if s.SecretSource == nil {
		return errors.New("radius: nil SecretSource")
	}

	addrStr := ":1812"
	if s.Addr != "" {
		addrStr = s.Addr
	}

	network := "udp"
	if s.Network != "" {
		network = s.Network
	}

	pc, err := net.ListenPacket(network, addrStr)
	if err != nil {
		return err
	}
	defer pc.Close()

	if s.ReadBufferSize > 0 {
		if udpConn, ok := pc.(*net.UDPConn); ok {
			_ = udpConn.SetReadBuffer(s.ReadBufferSize)
		}
	}

	return s.Serve(pc)
}

// Shutdown gracefully stops the server. It first closes all listeners and then
// waits for any running handlers to complete.
//
// Shutdown returns after nil all handlers have completed. ctx.Err() is
// returned if ctx is canceled.
//
// Any Serve methods return ErrShutdown after Shutdown is called.
func (s *PacketServer) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	s.initLocked()
	if atomic.CompareAndSwapInt32(&s.shutdownRequested, 0, 1) {
		for listener := range s.listeners {
			listener.Close()
		}

		s.ctxDone()
		s.activeDone()
	}
	s.mu.Unlock()

	select {
	case <-s.lastActive:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
