package script

import (
	"os"
	"testing"
	"time"

	"github.com/meklis/all-ok-radius-server/logger"
	"github.com/meklis/all-ok-radius-server/radius/events"
)

func testEngine(t *testing.T, path string) *Engine {
	t.Helper()
	lg, err := logger.New("test", 0, os.Stdout)
	if err != nil {
		t.Fatalf("logger.New: %v", err)
	}
	e, err := New(path, 2, time.Second, lg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return e
}

func TestCallAuthorize(t *testing.T) {
	e := testEngine(t, "examples/auth.lua")

	resp, err := e.CallAuthorize(&events.AuthRequest{
		NasIp:     "10.0.0.1",
		DeviceMac: "AA:BB:CC:DD:EE:FF",
	})
	if err != nil {
		t.Fatalf("CallAuthorize: %v", err)
	}
	if resp.PoolName != "default" {
		t.Errorf("expected pool_name=default, got %q", resp.PoolName)
	}
	if resp.LeaseTimeSec != 3600 {
		t.Errorf("expected lease_time_sec=3600, got %v", resp.LeaseTimeSec)
	}
}

func TestCallAuthorizeError(t *testing.T) {
	e := testEngine(t, "examples/auth.lua")

	_, err := e.CallAuthorize(&events.AuthRequest{
		NasIp:     "10.0.0.1",
		DeviceMac: "",
	})
	if err == nil {
		t.Fatal("expected error for empty device_mac, got nil")
	}
}

func TestCallAccounting(t *testing.T) {
	e := testEngine(t, "examples/acct.lua")

	err := e.CallAccounting(&events.AcctRequest{
		NasIp:      "10.0.0.1",
		DeviceMac:  "AA:BB:CC:DD:EE:FF",
		StatusType: "Start",
	})
	if err != nil {
		t.Fatalf("CallAccounting: %v", err)
	}
}

func TestCallPostAuth(t *testing.T) {
	e := testEngine(t, "examples/post_auth.lua")

	err := e.CallPostAuth(&events.AuthRequest{
		DeviceMac: "AA:BB:CC:DD:EE:FF",
	}, &events.AuthResponse{
		PoolName: "default",
	})
	if err != nil {
		t.Fatalf("CallPostAuth: %v", err)
	}
}

func TestConcurrentAuthorize(t *testing.T) {
	e := testEngine(t, "examples/auth.lua")

	done := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_, err := e.CallAuthorize(&events.AuthRequest{
				NasIp:     "10.0.0.1",
				DeviceMac: "AA:BB:CC:DD:EE:FF",
			})
			done <- err
		}()
	}
	for i := 0; i < 10; i++ {
		if err := <-done; err != nil {
			t.Errorf("concurrent CallAuthorize: %v", err)
		}
	}
}
