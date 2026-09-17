package clientdb

import (
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"
)

func TestReloadNoLeak(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("type") {
		case "devices":
			for i := 0; i < 5000; i++ {
				w.Write([]byte("33686018;085A119465E0;dlink\n"))
			}
		case "clients":
			for i := 0; i < 20000; i++ {
				w.Write([]byte("16909060;744D280EE846;085A119465E0;3\n"))
			}
		}
	}))
	defer srv.Close()

	s, err := New(Config{
		DevicesURL:      srv.URL + "?type=devices",
		Binds:           map[string]string{"clients": srv.URL + "?type=clients"},
		RefreshInterval: time.Hour,
	}, testLogger(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer s.Close()

	goroutinesBefore := runtime.NumGoroutine()

	runtime.GC()
	var m0 runtime.MemStats
	runtime.ReadMemStats(&m0)

	const iterations = 30
	for i := 0; i < iterations; i++ {
		if err := s.reload(); err != nil {
			t.Fatalf("reload #%d: %v", i, err)
		}
	}

	runtime.GC()
	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	goroutinesAfter := runtime.NumGoroutine()

	t.Logf("HeapAlloc before=%d after=%d iterations=%d", m0.HeapAlloc, m1.HeapAlloc, iterations)
	t.Logf("goroutines before=%d after=%d", goroutinesBefore, goroutinesAfter)

	// после 30 полных перезагрузок памяти не должно остаться существенно больше,
	// чем требует ОДИН живой снапшот - допуск x3 на фрагментацию/GC-паузы
	if m1.HeapAlloc > m0.HeapAlloc*3+10*1024*1024 {
		t.Errorf("подозрение на утечку: HeapAlloc вырос с %d до %d после %d reload()", m0.HeapAlloc, m1.HeapAlloc, iterations)
	}
	if goroutinesAfter > goroutinesBefore+2 {
		t.Errorf("подозрение на утечку горутин: было %d, стало %d", goroutinesBefore, goroutinesAfter)
	}
}
