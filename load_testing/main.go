// Command load_testing - нагрузочный клиент для радиус-сервера этого репозитория.
//
// Отправляет Access-Request пакеты, случайно выбирая один из профилей NAS/релея
// (см. sites.go, собраны по образцу реального трафика: CCR/DHCP-relay/NAT свитчи
// с их NAS-Identifier, Called-Station-Id, option-82 и т.д.) и случайный мак
// клиента на каждый запрос, чтобы каждый запрос выглядел как новое устройство.
package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"layeh.com/radius"
)

type Options struct {
	ServerAddr      string
	Secret          string
	Concurrency     int
	Count           int
	Duration        time.Duration
	RPS             int
	ExtraSites      int
	Profile         string
	Timeout         time.Duration
	DupVendor       bool
	MsgAuthPercent  int
	CalledStationID string // для -profile=simple

	SwitchMacs         [][6]byte
	KnownDevicePercent int

	MacPoolSize   int
	ClientMacPool [][6]byte
}

func parseFlags() Options {
	var o Options
	var durationStr string
	flag.StringVar(&o.ServerAddr, "server", "localhost:1812", "Адрес радиус-сервера host:port")
	flag.StringVar(&o.Secret, "secret", "secret", "Radius secret")
	flag.IntVar(&o.Concurrency, "concurrency", 50, "Число параллельных воркеров")
	flag.IntVar(&o.Count, "c", 100000, "Общее число запросов (игнорируется, если задан -duration)")
	flag.StringVar(&durationStr, "duration", "", "Длительность теста (например 30s, 5m). Если задано - -c игнорируется")
	flag.IntVar(&o.RPS, "rps", 0, "Ограничение суммарного RPS (0 = без ограничения, максимум скорости)")
	flag.IntVar(&o.ExtraSites, "sites", 50, "Количество доп. синтетических NAS-профилей (сверх примеров из задачи)")
	flag.StringVar(&o.Profile, "profile", "realistic", "Профиль пакета: realistic (полный набор атрибутов как в реальном трафике) или simple (минимальный пакет)")
	flag.DurationVar(&o.Timeout, "timeout", 2*time.Second, "Таймаут ожидания ответа на один запрос")
	flag.BoolVar(&o.DupVendor, "dup-vendor", true, "Дублировать option-82 под vendor 3561, как это делают реальные свитчи")
	flag.IntVar(&o.MsgAuthPercent, "msg-auth-percent", 30, "Процент запросов с Message-Authenticator атрибутом (0-100)")
	flag.StringVar(&o.CalledStationID, "called-station-id", "local", "Called-Station-Id для -profile=simple")
	var switchMacsStr string
	flag.StringVar(&switchMacsStr, "switch-macs", "", "Список реальных маков свитчей через запятую (AA:BB:CC:DD:EE:FF) для проверки известного в clientdb устройства")
	flag.IntVar(&o.KnownDevicePercent, "known-device-percent", 100, "Процент запросов с remote-id из -switch-macs (без -switch-macs не действует)")
	flag.IntVar(&o.MacPoolSize, "mac-pool", 0, "Число различных маков абонентских устройств (0 = каждый запрос со своим случайным маком, как новое устройство)")
	flag.Parse()

	if durationStr != "" {
		d, err := time.ParseDuration(durationStr)
		if err != nil {
			fmt.Printf("некорректное значение -duration=%v: %v\n", durationStr, err)
			os.Exit(1)
		}
		o.Duration = d
	}
	if switchMacsStr != "" {
		macs, err := parseMacs(switchMacsStr)
		if err != nil {
			fmt.Printf("некорректное значение -switch-macs: %v\n", err)
			os.Exit(1)
		}
		o.SwitchMacs = macs
	}
	return o
}

func parseMacs(s string) ([][6]byte, error) {
	var out [][6]byte
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		hwAddr, err := net.ParseMAC(part)
		if err != nil || len(hwAddr) != 6 {
			return nil, fmt.Errorf("%q: %v", part, err)
		}
		var mac [6]byte
		copy(mac[:], hwAddr)
		out = append(out, mac)
	}
	return out, nil
}

type Stats struct {
	Total, Success, Failed       int64
	SumNanos, MinNanos, MaxNanos int64
}

func (s *Stats) record(latency time.Duration, success bool) {
	atomic.AddInt64(&s.Total, 1)
	if success {
		atomic.AddInt64(&s.Success, 1)
	} else {
		atomic.AddInt64(&s.Failed, 1)
	}
	n := latency.Nanoseconds()
	atomic.AddInt64(&s.SumNanos, n)
	for {
		cur := atomic.LoadInt64(&s.MaxNanos)
		if n <= cur || atomic.CompareAndSwapInt64(&s.MaxNanos, cur, n) {
			break
		}
	}
	for {
		cur := atomic.LoadInt64(&s.MinNanos)
		if cur != 0 && n >= cur || atomic.CompareAndSwapInt64(&s.MinNanos, cur, n) {
			break
		}
	}
}

func (s *Stats) snapshot() (total, success, failed int64, avgMs, minMs, maxMs float64) {
	total = atomic.LoadInt64(&s.Total)
	success = atomic.LoadInt64(&s.Success)
	failed = atomic.LoadInt64(&s.Failed)
	sum := atomic.LoadInt64(&s.SumNanos)
	if total > 0 {
		avgMs = float64(sum) / float64(total) / 1e6
	}
	minMs = float64(atomic.LoadInt64(&s.MinNanos)) / 1e6
	maxMs = float64(atomic.LoadInt64(&s.MaxNanos)) / 1e6
	return
}

func worker(ctx context.Context, opts Options, sites []Site, stats *Stats, seed int64) {
	rnd := rand.New(rand.NewSource(seed))

	var ticker *time.Ticker
	if opts.RPS > 0 {
		interval := time.Duration(float64(time.Second) * float64(opts.Concurrency) / float64(opts.RPS))
		if interval <= 0 {
			interval = time.Nanosecond
		}
		ticker = time.NewTicker(interval)
		defer ticker.Stop()
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if opts.Count > 0 && opts.Duration == 0 && atomic.LoadInt64(&stats.Total) >= int64(opts.Count) {
			return
		}
		if ticker != nil {
			select {
			case <-ticker.C:
			case <-ctx.Done():
				return
			}
		}

		var pkt *radius.Packet
		if opts.Profile == "simple" {
			pkt = buildSimplePacket(opts.Secret, opts.CalledStationID, opts.ClientMacPool, rnd)
		} else {
			site := sites[rnd.Intn(len(sites))]
			pkt = buildRealisticPacket(opts.Secret, site, opts, rnd)
		}

		reqCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
		start := time.Now()
		resp, err := radius.Exchange(reqCtx, pkt, opts.ServerAddr)
		latency := time.Since(start)
		cancel()

		success := err == nil && resp != nil && resp.Code == radius.CodeAccessAccept
		stats.record(latency, success)
	}
}

func reportLoop(ctx context.Context, stats *Stats, start time.Time) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var lastTotal int64
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			total, success, failed, avgMs, _, _ := stats.snapshot()
			rps := total - lastTotal
			lastTotal = total
			fmt.Printf("[%6.0fs] total=%d rps=%d success=%d failed=%d avg_latency=%.2fms\n",
				time.Since(start).Seconds(), total, rps, success, failed, avgMs)
		}
	}
}

func main() {
	opts := parseFlags()

	seedRnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	sites := buildSitePool(opts.ExtraSites, seedRnd)
	if opts.MacPoolSize > 0 {
		opts.ClientMacPool = make([][6]byte, opts.MacPoolSize)
		for i := range opts.ClientMacPool {
			opts.ClientMacPool[i] = randMAC(seedRnd)
		}
	}

	fmt.Printf(`Нагрузочное тестирование радиус-сервера
Server:        %v
Profile:       %v
Concurrency:   %v
RPS limit:     %v
NAS profiles:  %v
`, opts.ServerAddr, opts.Profile, opts.Concurrency, opts.RPS, len(sites))
	if opts.Duration > 0 {
		fmt.Printf("Duration:      %v\n", opts.Duration)
	} else {
		fmt.Printf("Count:         %v\n", opts.Count)
	}
	fmt.Println()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nПолучен сигнал остановки, завершаю тест...")
		cancel()
	}()

	if opts.Duration > 0 {
		time.AfterFunc(opts.Duration, cancel)
	}

	stats := &Stats{}
	start := time.Now()
	go reportLoop(ctx, stats, start)

	done := make(chan struct{}, opts.Concurrency)
	for i := 0; i < opts.Concurrency; i++ {
		go func(seed int64) {
			worker(ctx, opts, sites, stats, seed)
			done <- struct{}{}
		}(time.Now().UnixNano() + int64(i)*7919)
	}
	for i := 0; i < opts.Concurrency; i++ {
		<-done
	}

	elapsed := time.Since(start)
	total, success, failed, avgMs, minMs, maxMs := stats.snapshot()
	avgRps := float64(total) / elapsed.Seconds()
	fmt.Printf(`
=========================================================
================== Результат теста =====================
=========================================================
Длительность:              %v
Запросов всего:             %v
Успешных ответов:           %v
Ошибок/неуспешных:          %v
RPS (среднее):               %.1f
Latency (min/avg/max) ms:    %.2f/%.2f/%.2f
==========================================================
`, elapsed.Round(time.Millisecond), total, success, failed, avgRps, minMs, avgMs, maxMs)
}
