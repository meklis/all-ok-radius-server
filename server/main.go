package main

import (
	"flag"
	"fmt"
	"github.com/meklis/all-ok-radius-server/api"
	"github.com/meklis/all-ok-radius-server/config"
	"github.com/meklis/all-ok-radius-server/logger"
	"github.com/meklis/all-ok-radius-server/prom"
	"github.com/meklis/all-ok-radius-server/radius"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/ztrue/tracerr"
	"net/http"
)

var (
	Config     config.Configuration
	pathConfig string
	lg         *logger.Logger
)

const (
	VERSION    = "0.1"
	BUILD_DATE = "2020-05-19"
)

func init() {
	flag.StringVar(&pathConfig, "c", "radius.server.conf.yml", "Configuration file for radius-server")
	flag.Parse()
}

func main() {
	fmt.Println("Initialize radius-server v0.1 ...")

	//Load configuration
	if err := config.LoadConfig(pathConfig, &Config); err != nil {
		panic(err)
	}
	//Configure logger from cronfiguration
	lg = config.ConfigureLogger(&Config)

	//Initialize prometheus
	if Config.Prometheus.Enabled {
		prom.PromEnabled = true
		lg.NoticeF("Exporter for prometheus is enabled...")
		http.Handle(Config.Prometheus.Path, promhttp.Handler())
		go func() {
			err := http.ListenAndServe(fmt.Sprintf(":%v", Config.Prometheus.Port), nil)
			lg.CriticalF("Prometheus exporter critical err: %v", err)
			panic(err)
		}()
		lg.NoticeF("Prometheus exporter started on 0.0.0.0:%v%v", Config.Prometheus.Port, Config.Prometheus.Path)
		prom.SysInfo(VERSION, BUILD_DATE)
	}

	//Initialize API
	apiInstance := api.Init(Config.Api, lg)

	//Initialize server
	rad := radius.Init()
	err := rad.SetAPI(apiInstance).
		SetAgentParsing(Config.Radius.AgentParsingEnabled).
		SetListenAddr(Config.Radius.ListenAddr).
		SetLogger(lg).
		SetSecret(Config.Radius.Secret).
		ListenAndServe()
	if err != nil {
		panic(tracerr.Sprint(err))
	}
}
