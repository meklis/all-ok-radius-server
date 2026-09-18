package clientdb

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/meklis/all-ok-radius-server/logger"
)

// clients - формат ip;client_mac;device_mac;port
const clientsSample = `33686018;744D280EE846;085A119465E0;3
33686018;AABBCCDDEEFF;085A119465E0;6
33686018;AABBCCDDEEFF;085A119465E0;7
2887141235;1C61B459F78F;085A11946600;6
`

// smart - формат ip;client_mac (без устройства и порта)
const smartSample = `33686018;744D280EE846
169088289;0418D6EE60F1
`

const devicesSample = `33686018;085A119465E0;dlink
33686018;085A11946600;dlink
`

func testLogger(t *testing.T) *logger.Logger {
	t.Helper()
	lg, err := logger.New("test", 0, os.Stdout)
	if err != nil {
		t.Fatalf("logger.New: %v", err)
	}
	return lg
}

func testServer(t *testing.T, devices, clients, smart string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("type") {
		case "devices":
			w.Write([]byte(devices))
		case "clients":
			w.Write([]byte(clients))
		case "smart":
			w.Write([]byte(smart))
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
}

func testConfig(url string) Config {
	return Config{
		DevicesURL: url + "?type=devices",
		Binds: map[string]string{
			"clients": url + "?type=clients",
			"smart":   url + "?type=smart",
		},
		RefreshInterval: time.Hour,
	}
}

func TestNewAndLookups(t *testing.T) {
	srv := testServer(t, devicesSample, clientsSample, smartSample)
	defer srv.Close()

	s, err := New(testConfig(srv.URL), testLogger(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer s.Close()

	d, ok := s.GetDeviceByMac("08:5a:11:94:65:e0")
	if !ok {
		t.Fatal("device not found")
	}
	if d.IP.String() != "2.2.2.2" || d.ParseType != "dlink" {
		t.Errorf("unexpected device: %+v", d)
	}
	if _, ok := s.GetDeviceByMac("00:00:00:00:00:00"); ok {
		t.Error("unexpected device found")
	}

	// smart - поиск только по мак клиента (устройство/порт в этом источнике отсутствуют)
	binds := s.GetBind("smart", "744D280EE846", "", "")
	if len(binds) != 1 || binds[0].IP.String() != "2.2.2.2" {
		t.Errorf("unexpected smart binds: %+v", binds)
	}

	// clients - несколько записей под одним мак клиента (разные устройства/порты)
	binds = s.GetBind("clients", "AABBCCDDEEFF", "", "")
	if len(binds) != 2 {
		t.Fatalf("expected 2 binds for shared mac, got %v: %+v", len(binds), binds)
	}

	// уточнение мак устройства + портом сужает до одной записи
	binds = s.GetBind("clients", "AA:BB:CC:DD:EE:FF", "085a119465e0", "6")
	if len(binds) != 1 || binds[0].Port != 6 {
		t.Errorf("unexpected bind by mac+device+port: %+v", binds)
	}

	// несуществующий порт не должен найтись при уточнении
	if binds := s.GetBind("clients", "744D280EE846", "085A119465E0", "99"); len(binds) != 0 {
		t.Errorf("expected no binds for wrong port, got %+v", binds)
	}

	// неизвестное имя базы - пустой результат, не паника
	if binds := s.GetBind("unknown-db", "744D280EE846", "", ""); len(binds) != 0 {
		t.Errorf("expected empty result for unknown db, got %+v", binds)
	}

	if binds := s.GetBind("clients", "unknown-mac", "", ""); len(binds) != 0 {
		t.Errorf("expected empty result for unknown mac, got %+v", binds)
	}
}

func TestReloadClearsStaleRecords(t *testing.T) {
	devices := devicesSample
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("type") {
		case "devices":
			w.Write([]byte(devices))
		case "clients", "smart":
			w.Write([]byte(""))
		}
	}))
	defer srv.Close()

	s, err := New(testConfig(srv.URL), testLogger(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer s.Close()

	if _, ok := s.GetDeviceByMac("085A119465E0"); !ok {
		t.Fatal("expected device to be present before reload")
	}

	devices = "" // источник больше не отдаёт эту запись
	if err := s.reload(); err != nil {
		t.Fatalf("reload: %v", err)
	}

	if _, ok := s.GetDeviceByMac("085A119465E0"); ok {
		t.Error("stale record was not cleared after reload")
	}
}

func TestNewRequiresDevicesURL(t *testing.T) {
	if _, err := New(Config{}, testLogger(t)); err == nil {
		t.Fatal("expected error for empty devices_url")
	}
}
