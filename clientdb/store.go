package clientdb

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/meklis/all-ok-radius-server/logger"
)

// Config - параметры загрузки внешней базы устройств и привязок.
// Используется только скриптами (lua), api про этот пакет не знает.
// devices_url обязателен, binds - произвольный набор именованных источников
// (например clients/smart), доступных из lua через db:getBind(name, ...)
type Config struct {
	DevicesURL      string            `yaml:"devices_url"`
	Binds           map[string]string `yaml:"binds"`
	RefreshInterval time.Duration     `yaml:"refresh_interval"`
	Timeout         time.Duration     `yaml:"timeout"`
}

type Device struct {
	IP        net.IP
	Mac       string
	ParseType string
}

type Bind struct {
	IP        net.IP
	ClientMac string
	DeviceMac string
	Port      int
}

// bindIndex - один именованный источник привязок (например "clients" или "smart"),
// проиндексированный под разные варианты вызова GetBind, включая поиск без мак-адреса
// клиента (device+port) - нужен для портов, отданных под общий пул независимо от того,
// чей мак сейчас на них подключен. Значение - слайс, т.к. под одним ключом может быть
// несколько записей (например клиент подключён через разные устройства/порты)
type bindIndex struct {
	byMac           map[string][]*Bind
	byMacDevice     map[string][]*Bind
	byMacDevicePort map[string][]*Bind
	byDevicePort    map[string][]*Bind
	count           int // число записей, разобранных из источника (для лога)
}

type snapshot struct {
	devices map[string]Device
	binds   map[string]*bindIndex
}

// Store - потокобезопасное хранилище с периодическим обновлением из HTTP-источников.
// Чтение (Get*) не блокируется обновлением - снапшот подменяется атомарно целиком.
type Store struct {
	conf   Config
	lg     *logger.Logger
	client *http.Client
	snap   atomic.Value // *snapshot
	stop   chan struct{}
}

// New синхронно загружает базу при старте (fail-fast) и запускает фоновое обновление
func New(conf Config, lg *logger.Logger) (*Store, error) {
	if conf.DevicesURL == "" {
		return nil, fmt.Errorf("clientdb: devices_url не задан")
	}
	if conf.RefreshInterval <= 0 {
		conf.RefreshInterval = 5 * time.Minute
	}
	if conf.Timeout <= 0 {
		conf.Timeout = 30 * time.Second
	}

	s := &Store{
		conf:   conf,
		lg:     lg,
		client: &http.Client{Timeout: conf.Timeout},
		stop:   make(chan struct{}),
	}

	if err := s.reload(); err != nil {
		return nil, err
	}
	go s.loop()
	return s, nil
}

// Close останавливает фоновое обновление
func (s *Store) Close() { close(s.stop) }

func (s *Store) loop() {
	ticker := time.NewTicker(s.conf.RefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := s.reload(); err != nil {
				s.lg.ErrorF("clientdb: обновление базы не удалось, оставлены старые данные: %v", err)
			}
		case <-s.stop:
			return
		}
	}
}

func (s *Store) reload() error {
	s.lg.NoticeF("clientdb: start loading database")

	devices, err := s.loadDevices(s.conf.DevicesURL)
	if err != nil {
		return fmt.Errorf("devices: %w", err)
	}

	binds := make(map[string]*bindIndex, len(s.conf.Binds))
	for name, url := range s.conf.Binds {
		idx, err := s.loadBindDB(url)
		if err != nil {
			return fmt.Errorf("binds.%v: %w", name, err)
		}
		binds[name] = idx
	}

	s.snap.Store(&snapshot{devices: devices, binds: binds})
	s.lg.NoticeF("clientdb: база обновлена: devices=%v", len(devices))
	for name, idx := range binds {
		s.lg.NoticeF("clientdb: binds.%v=%v", name, idx.count)
	}
	return nil
}

func (s *Store) fetchURL(url string) (io.ReadCloser, error) {
	resp, err := s.client.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected status: %v", resp.Status)
	}
	return resp.Body, nil
}

// loadDevices - строки вида ip_int;device_mac;parse_type, ключ - device_mac
func (s *Store) loadDevices(url string) (map[string]Device, error) {
	body, err := s.fetchURL(url)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	result := make(map[string]Device)
	scanner := newScanner(body)
	for scanner.Scan() {
		fields := strings.Split(strings.TrimSpace(scanner.Text()), ";")
		if len(fields) < 3 {
			continue
		}
		mac := normalizeMac(fields[1])
		if mac == "" {
			continue
		}
		result[mac] = Device{
			IP:        intToIP(fields[0]),
			Mac:       mac,
			ParseType: strings.TrimSpace(fields[2]),
		}
	}
	return result, scanner.Err()
}

// loadBindDB разбирает один именованный источник привязок. Поддерживаются форматы строк:
//
//	ip;client_mac                      - например smart
//	ip;client_mac;device_mac;port      - например clients
func (s *Store) loadBindDB(url string) (*bindIndex, error) {
	body, err := s.fetchURL(url)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	idx := &bindIndex{
		byMac:           make(map[string][]*Bind),
		byMacDevice:     make(map[string][]*Bind),
		byMacDevicePort: make(map[string][]*Bind),
		byDevicePort:    make(map[string][]*Bind),
	}

	scanner := newScanner(body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		b, ok := parseBindFields(strings.Split(line, ";"))
		if !ok {
			continue
		}
		idx.count++
		idx.byMac[b.ClientMac] = append(idx.byMac[b.ClientMac], b)
		if b.DeviceMac != "" {
			key := b.ClientMac + "|" + b.DeviceMac
			idx.byMacDevice[key] = append(idx.byMacDevice[key], b)
			if b.Port != 0 {
				key = key + "|" + strconv.Itoa(b.Port)
				idx.byMacDevicePort[key] = append(idx.byMacDevicePort[key], b)

				dpKey := b.DeviceMac + "|" + strconv.Itoa(b.Port)
				idx.byDevicePort[dpKey] = append(idx.byDevicePort[dpKey], b)
			}
		}
	}
	return idx, scanner.Err()
}

func parseBindFields(fields []string) (*Bind, bool) {
	if len(fields) < 2 {
		return nil, false
	}
	clientMac := normalizeMac(fields[1])
	if clientMac == "" {
		return nil, false
	}
	b := &Bind{IP: intToIP(fields[0]), ClientMac: clientMac}
	if len(fields) >= 4 {
		b.DeviceMac = normalizeMac(fields[2])
		b.Port, _ = strconv.Atoi(strings.TrimSpace(fields[3]))
	}
	return b, true
}

func newScanner(r io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	return scanner
}

// currentSnapshot - снапшот всегда есть: New() возвращает *Store только после
// успешного первого reload(), а reload() либо записывает снапшот целиком, либо
// возвращает ошибку до записи - Store с пустым/nil снапшотом наружу не попадает
func (s *Store) currentSnapshot() *snapshot {
	return s.snap.Load().(*snapshot)
}

func (s *Store) GetDeviceByMac(mac string) (Device, bool) {
	d, ok := s.currentSnapshot().devices[normalizeMac(mac)]
	return d, ok
}

// GetBind ищет привязки в именованном источнике dbName (см. Config.Binds).
// mac - мак-адрес клиента, deviceMac/port - опциональные уточнения. mac тоже может
// быть пустым, если заданы deviceMac+port - тогда ищутся все привязки на этом порту
// устройства независимо от мак-адреса клиента (порт, отданный под общий пул).
// Возвращает все совпадения - пустой слайс, если ничего не найдено
// (в т.ч. если такого dbName нет вовсе)
func (s *Store) GetBind(dbName, mac, deviceMac, port string) []Bind {
	idx, ok := s.currentSnapshot().binds[dbName]
	if !ok {
		return nil
	}

	mac = normalizeMac(mac)
	deviceMac = normalizeMac(deviceMac)
	port = normalizePort(port)

	var ptrs []*Bind
	switch {
	case mac != "" && deviceMac != "" && port != "":
		ptrs = idx.byMacDevicePort[mac+"|"+deviceMac+"|"+port]
	case mac != "" && deviceMac != "":
		ptrs = idx.byMacDevice[mac+"|"+deviceMac]
	case mac != "":
		ptrs = idx.byMac[mac]
	case deviceMac != "" && port != "":
		ptrs = idx.byDevicePort[deviceMac+"|"+port]
	}
	if len(ptrs) == 0 {
		return nil
	}

	result := make([]Bind, len(ptrs))
	for i, p := range ptrs {
		result[i] = *p
	}
	return result
}

func normalizeMac(s string) string {
	s = strings.ToUpper(s)
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if (r >= '0' && r <= '9') || (r >= 'A' && r <= 'F') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func normalizePort(port string) string {
	port = strings.TrimSpace(port)
	if port == "" {
		return ""
	}
	if n, err := strconv.Atoi(port); err == nil {
		return strconv.Itoa(n)
	}
	return port
}

func intToIP(s string) net.IP {
	n, err := strconv.ParseUint(strings.TrimSpace(s), 10, 32)
	if err != nil {
		return nil
	}
	ip := make(net.IP, 4)
	binary.BigEndian.PutUint32(ip, uint32(n))
	return ip
}
