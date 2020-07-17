package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/cheggaaa/pb/v3"
	"layeh.com/radius"
	"layeh.com/radius/rfc2865"
	"math/rand"
	"sync"
	"time"
)

type Flags struct {
	ServerAddr     string
	Concurency     int
	Count          int
	Secret         string
	DhcpServerName string
	MacAddress     string
	GenerateMac    string
}

type Response struct {
	Latency   float64
	IsSuccess bool
}

type Statistic struct {
	FailedRequests  int
	SuccessRequests int
	LatencyMin      float64
	SummaryLatency  float64
	LatencyMax      float64
	MinRPS          int
	MaxRPS          int
	SummaryRPS      int
	CountRPSTickers int
	sync.Mutex
}

var (
	flags    Flags
	stopFlag = make(chan interface{}, 1)
	statuses = make(chan Response, 100000)
	stat     Statistic
)

func init() {
	flag.StringVar(&flags.Secret, "secret", "secret", "Secret")
	flag.StringVar(&flags.ServerAddr, "server", "localhost:1812", "Server address")
	flag.StringVar(&flags.DhcpServerName, "server-name", "local", "DHCP server name from mikrotik")
	flag.StringVar(&flags.MacAddress, "mac-address", "**:**:**:**:**:**", `Mac Address for test. 
MAC can be setted with * for auto generated 0-F symbol, for testing without caching
`)
	flag.IntVar(&flags.Count, "c", 100, "Count of requests")
	flag.IntVar(&flags.Concurency, "concurency", 10, "Concurency")
	flag.Parse()
}

func generateMac(mac string) string {
	min := 0
	max := 16
	macInBytes := []byte(mac)
	rand.Seed(time.Now().UnixNano())
	for num, m := range macInBytes {
		if string(m) == "*" {
			macInBytes[num] = byte(fmt.Sprintf("%X", rand.Intn(max-min+1)+min)[0])
		}
	}
	return string(macInBytes)
}

func generatePacket() *radius.Packet {
	packet := radius.New(radius.CodeAccessRequest, []byte(flags.Secret))
	macAddr := generateMac(flags.MacAddress)
	rfc2865.UserName_SetString(packet, macAddr)
	rfc2865.CalledStationID_SetString(packet, flags.DhcpServerName)
	rfc2865.UserPassword_SetString(packet, "")
	return packet
}

func main() {
	fmt.Println("Start testing...")
	bar := pb.StartNew(flags.Count)
	ticker := time.NewTicker(time.Millisecond * 1000)
	go func() {
		latestRequest := 0
		for range ticker.C {
			stat.Lock()
			rps := (stat.FailedRequests + stat.SuccessRequests) - latestRequest
			latestRequest = (stat.FailedRequests + stat.SuccessRequests)
			if rps > stat.MaxRPS {
				stat.MaxRPS = rps
			}
			if rps < stat.MinRPS {
				stat.MinRPS = rps
			}
			stat.SummaryRPS += rps
			stat.CountRPSTickers++
			stat.Unlock()
		}
	}()
	go func() {
		for {
			if flags.Count <= stat.SuccessRequests+stat.FailedRequests {
				stopFlag <- true
				return
			}
			bar.Increment()
			resp := <-statuses
			if resp.IsSuccess {
				stat.SuccessRequests++
			} else {
				stat.FailedRequests++
			}
			if stat.LatencyMax < resp.Latency {
				stat.LatencyMax = resp.Latency
			}
			if stat.LatencyMin > resp.Latency {
				stat.LatencyMin = resp.Latency
			}
			stat.SummaryLatency += resp.Latency
		}
	}()
	for i := 0; i < flags.Concurency; i++ {
		go func() {
			for {
				start := time.Now().UnixNano()
				response, err := radius.Exchange(context.Background(), generatePacket(), flags.ServerAddr)
				if err != nil {
					statuses <- Response{
						Latency:   float64(time.Now().UnixNano()-start) / float64(time.Duration(time.Second)),
						IsSuccess: false,
					}
					continue
				}
				if response.Code == radius.CodeAccessAccept {
					statuses <- Response{
						Latency:   float64(time.Now().UnixNano()-start) / float64(time.Duration(time.Second)),
						IsSuccess: true,
					}
				} else {
					statuses <- Response{
						Latency:   float64(time.Now().UnixNano()-start) / float64(time.Duration(time.Second)),
						IsSuccess: false,
					}
				}
				if len(stopFlag) != 0 {
					return
				}
			}
		}()
	}
	for {
		if len(stopFlag) != 0 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	bar.Finish()
	avgLatency := stat.SummaryLatency / float64(stat.SuccessRequests+stat.FailedRequests)
	avgRps := float64(stat.SummaryRPS) / float64(stat.CountRPSTickers)
	fmt.Printf(`
=========================================================
================== Result of testing ====================
=========================================================
Requests:				%v
Success responces:			%v
Failed responces:			%v

RPS (Avg/Max)		%.2f/%v
Latencies (Min/Avg/Max) %.2f/%.2f/%.2f
==========================================================
`, (stat.SuccessRequests + stat.FailedRequests), stat.SuccessRequests, stat.FailedRequests,
		avgRps, stat.MaxRPS,
		stat.LatencyMin, avgLatency, stat.LatencyMax)

}
