package script

import (
	"os"
	"testing"
	"time"

	"github.com/meklis/all-ok-radius-server/logger"
	"github.com/meklis/all-ok-radius-server/radius/events"
)

func testLogger(t *testing.T) *logger.Logger {
	t.Helper()
	lg, err := logger.New("test", 0, os.Stdout)
	if err != nil {
		t.Fatalf("logger.New: %v", err)
	}
	return lg
}

func TestNewProcessorAuthOnly(t *testing.T) {
	p, err := NewProcessor(Config{
		Auth:     "examples/auth.lua",
		PoolSize: 2,
		Timeout:  time.Second,
	}, testLogger(t))
	if err != nil {
		t.Fatalf("NewProcessor: %v", err)
	}
	if p.acctEngine != nil || p.postAuthEngine != nil {
		t.Fatal("acct/post_auth не заданы в конфиге, но движки созданы")
	}

	resp, err := p.Get(&events.AuthRequest{NasIp: "10.0.0.1", DeviceMac: "AA:BB:CC:DD:EE:FF"})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if resp.PoolName != "default" {
		t.Errorf("expected pool_name=default, got %q", resp.PoolName)
	}

	// без сконфигурированных acct/post_auth вызовы должны быть no-op, без паники
	p.SendAcct(&events.AcctRequest{DeviceMac: "AA:BB:CC:DD:EE:FF"})
	p.SendPostAuth(events.AuthRequest{DeviceMac: "AA:BB:CC:DD:EE:FF"}, events.AuthResponse{PoolName: "default"})
}

func TestNewProcessorRequiresAuth(t *testing.T) {
	_, err := NewProcessor(Config{}, testLogger(t))
	if err == nil {
		t.Fatal("expected error when script.auth is empty")
	}
}

func TestNewProcessorAllPhases(t *testing.T) {
	p, err := NewProcessor(Config{
		Auth:     "examples/auth.lua",
		Acct:     "examples/acct.lua",
		PostAuth: "examples/post_auth.lua",
		PoolSize: 2,
		Timeout:  time.Second,
	}, testLogger(t))
	if err != nil {
		t.Fatalf("NewProcessor: %v", err)
	}

	p.SendAcct(&events.AcctRequest{DeviceMac: "AA:BB:CC:DD:EE:FF", StatusType: "Start"})
	p.SendPostAuth(events.AuthRequest{DeviceMac: "AA:BB:CC:DD:EE:FF"}, events.AuthResponse{PoolName: "default"})
	time.Sleep(50 * time.Millisecond)
}
