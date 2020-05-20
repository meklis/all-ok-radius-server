package api

import (
	"github.com/meklis/all-ok-radius-server/radius/events"
	"time"
)

type ApiConfig struct {
	Addresses     []string `yaml:"addresses"`
	AliveChecking struct {
		Enabled        bool          `yaml:"enabled"`
		DisableTimeout time.Duration `yaml:"disable_timeout"`
	} `yaml:"alive_checking"`
	Timeout time.Duration `yaml:"timeout"`
	Caching struct {
		Enabled          bool          `yaml:"enabled"`
		ActualizeTimeout time.Duration `yaml:"actualize_timeout"`
		TimeoutExpires   time.Duration `yaml:"expire_timeout"`
	} `yaml:"caching"`
}

type ApiResponse struct {
	Data       events.Response `json:"data"`
	Meta       interface{}     `json:"meta"`
	StatusCode int             `json:"statusCode"`
}
