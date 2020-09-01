package prom

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	radRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "rad_request_count",
		Help: "Count of requests from NAS",
	}, []string{"host"})
	radAcctRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "rad_acct_requests_count",
		Help: "rad acct requests count",
	}, []string{"host", "server_name"})
	radRequestsIpAddressCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "rad_request_ip_count",
		Help: "Count of requests from NAS",
	}, []string{"host"})
	radRequestsIpPoolCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "rad_request_pool_count",
		Help: "Count of requests from NAS",
	}, []string{"host"})
	radRequestsCountByPool = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "rad_request_by_pool_count",
		Help: "Count of requests from NAS by pool name",
	}, []string{"host", "pool_name"})
	radCriticalCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "rad_critical_count",
		Help: "Errors stat",
	}, []string{"caller"})
	radWarningsCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "rad_warnings_count",
		Help: "Errors stat",
	}, []string{"caller"})
	radErrorsCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "rad_errors_count",
		Help: "Errors stat",
	}, []string{"caller"})
	cacheSize = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "rad_cache_responses_count",
		Help: "Count of responses in cache",
	}, []string{})
	apiAliveStatus = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "rad_api_alive_status",
		Help: "API alive status",
	}, []string{"api_addr"})
	apiPostAuthQueueLen = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "rad_api_post_auth_queue_len",
		Help: "Queue len for post auth",
	}, []string{})
	apiAcctQueueLen = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "rad_api_acct_queue_len",
		Help: "Queue len for acct",
	}, []string{})
	promSysInfo = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "rad_sys_version",
		Help: "Version of radius-server",
	}, []string{"version", "build_date"})
	radDetailedRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "rad_mac_server_count",
		Help: "Detailed requests count info by MAC - DHCP-server",
	}, []string{"host", "mac", "server_name", "response_type"})
	PromEnabled                bool
	PromDetailedMacInfoEnabled bool
)

type ErrLevel int

const Critical ErrLevel = 1
const Error ErrLevel = 2
const Warning ErrLevel = 3

func ErrorsInc(level ErrLevel, caller string) {
	if !PromEnabled {
		return
	}
	var pr *prometheus.CounterVec
	switch level {
	case Critical:
		pr = radCriticalCount
	case Error:
		pr = radErrorsCount
	case Warning:
		pr = radWarningsCount
	default:
		return
	}
	pr.With(map[string]string{"caller": caller}).Inc()
}

func RadRequestsInc(host string) {
	if !PromEnabled {
		return
	}
	radRequests.With(map[string]string{"host": host}).Inc()
}

func RadRequestsIpAddressInc(host string) {
	if !PromEnabled {
		return
	}
	radRequestsIpAddressCount.With(map[string]string{"host": host}).Inc()
}

func RadDetailedRequest(host, serverName, macAddr, responseType string) {
	if !PromEnabled || !PromDetailedMacInfoEnabled {
		return
	}
	radDetailedRequests.With(map[string]string{"host": host, "server_name": serverName, "mac": macAddr, "response_type": responseType}).Inc()
}

func RadRequestsPoolInc(host string) {
	if !PromEnabled {
		return
	}
	radRequestsIpPoolCount.With(map[string]string{"host": host}).Inc()
}
func RadAcctRequestsInc(host string, serverName string) {
	if !PromEnabled {
		return
	}
	radAcctRequests.With(map[string]string{"host": host, "server_name": serverName}).Inc()
}

func RadRequestsByPoolInc(host, poolName string) {
	if !PromEnabled {
		return
	}
	radRequestsCountByPool.With(map[string]string{"host": host, "pool_name": poolName}).Inc()
}

func SetCacheSize(size int) {
	if !PromEnabled {
		return
	}
	cacheSize.With(map[string]string{}).Set(float64(size))
}
func SetPostAuthQueueSize(size int) {
	if !PromEnabled {
		return
	}
	apiPostAuthQueueLen.With(map[string]string{}).Set(float64(size))
}
func SetAcctQueueSize(size int) {
	if !PromEnabled {
		return
	}
	apiAcctQueueLen.With(map[string]string{}).Set(float64(size))
}

func SetApiStatus(address string, alive bool) {
	if !PromEnabled {
		return
	}
	status := 1
	if !alive {
		status = 0
	}
	apiAliveStatus.With(map[string]string{"api_addr": address}).Set(float64(status))
}

func SysInfo(version string, buildDate string) {
	if !PromEnabled {
		return
	}
	promSysInfo.With(map[string]string{"version": version, "build_date": buildDate}).Inc()
}
