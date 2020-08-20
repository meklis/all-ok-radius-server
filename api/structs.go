package api

import (
	"github.com/meklis/all-ok-radius-server/radius/events"
	"time"
)

type ApiConfig struct {
	Auth struct {
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
	} `yaml:"auth"`
	PostAuth struct {
		Enabled   bool     `yaml:"enabled"`
		Addresses []string `yaml:"addresses"`
	} `yaml:"postauth"`
	Acct struct {
		Enabled   bool     `yaml:"enabled"`
		Addresses []string `yaml:"addresses"`
	} `yaml:"acct"`
	Timeout time.Duration `yaml:"timeout"`
}

type ApiResponse struct {
	Data       events.AuthResponse `json:"data"`
	Meta       interface{}         `json:"meta"`
	StatusCode int                 `json:"statusCode"`
}

type PostAuth struct {
	Request  events.AuthRequest  `json:"request"`
	Response events.AuthResponse `json:"response"`
}
