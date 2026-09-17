package radius

import (
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/meklis/all-ok-radius-server/logger"
	"layeh.com/radius"
)

type Radius struct {
	lg         *logger.Logger
	listenAddr string
	secret     string
	processor  Processor
	classId    int64
	sync.Mutex
}

func Init() *Radius {
	rad := new(Radius)
	rad.listenAddr = "0.0.0.0:1812"
	rad.secret = "secret"
	rad.lg, _ = logger.New("radius", 0, os.Stdout)
	rad.classId = time.Now().Unix()
	return rad
}

func (rad *Radius) getClassId() string {
	rad.Lock()
	defer rad.Unlock()
	rad.classId = rad.classId + 1
	return fmt.Sprintf("%v", rad.classId)
}

func (rad *Radius) SetLogger(lg *logger.Logger) *Radius {
	rad.lg = lg
	return rad
}
func (rad *Radius) SetListenAddr(listenAddr string) *Radius {
	rad.listenAddr = listenAddr
	return rad
}
func (rad *Radius) SetSecret(secret string) *Radius {
	rad.secret = secret
	return rad
}

func (rad *Radius) SetProcessor(p Processor) *Radius {
	rad.processor = p
	return rad
}

func (rad *Radius) ListenAndServe() error {
	server := radius.PacketServer{
		Addr:         rad.listenAddr,
		Network:      "udp",
		SecretSource: radius.StaticSecretSource([]byte(rad.secret)),
		Handler:      radius.HandlerFunc(rad.handler),
	}

	rad.logListenAddr()
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
		return err
	}
	return nil
}

// logListenAddr пишет порт и интерфейс(ы), на которых слушает радиус.
// Для 0.0.0.0/:: - перечисляет реальные адреса всех сетевых интерфейсов
func (rad *Radius) logListenAddr() {
	host, port, err := net.SplitHostPort(rad.listenAddr)
	if err != nil {
		rad.lg.InfoF("Starting radius server on %v", rad.listenAddr)
		return
	}

	if host != "" && host != "0.0.0.0" && host != "::" {
		rad.lg.InfoF("Starting radius server: interface=%v port=%v", host, port)
		return
	}

	rad.lg.InfoF("Starting radius server: interface=%v (all) port=%v", host, port)
	ifaces, err := net.Interfaces()
	if err != nil {
		return
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || ipNet.IP.IsLinkLocalUnicast() {
				continue
			}
			rad.lg.InfoF("  interface %v: %v", iface.Name, addr.String())
		}
	}
}
