package api

import (
	"github.com/meklis/all-ok-radius-server/radius/events"
	"time"
)

type ApiConfig struct {
	RadReply struct {
		Addresses     []string `yaml:"addresses"`
		AliveChecking struct {
			Enabled        bool          `yaml:"enabled"`
			DisableTimeout time.Duration `yaml:"disable_timeout"`
		} `yaml:"alive_checking"`
		Caching struct {
			Enabled          bool          `yaml:"enabled"`
			ActualizeTimeout time.Duration `yaml:"actualize_timeout"`
			TimeoutExpires   time.Duration `yaml:"expire_timeout"`
		} `yaml:"caching"`
	} `yaml:"radreply"`
	PostAuth struct {
		Enabled   bool     `yaml:"enabled"`
		Addresses []string `yaml:"addresses"`
	} `yaml:"postauth"`
	Timeout time.Duration `yaml:"timeout"`
}

type ApiResponse struct {
	Data       events.Response `json:"data"`
	Meta       interface{}     `json:"meta"`
	StatusCode int             `json:"statusCode"`
}

type PostAuth struct {
	Request  events.Request  `json:"request"`
	Response events.Response `json:"response"`
}
