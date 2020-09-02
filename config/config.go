package config

import (
	"fmt"
	"github.com/meklis/all-ok-radius-server/api"
	"github.com/meklis/all-ok-radius-server/logger"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"strings"
	"time"
)

type Configuration struct {
	Logger struct {
		Console struct {
			PrintFile   bool `yaml:"print_file"`
			EnableColor bool `yaml:"enable_color"`
			LogLevel    int  `yaml:"log_level"`
			Enabled     bool `yaml:"enabled"`
		} `yaml:"console"`
	} `yaml:"logger"`
	Prometheus struct {
		Enabled                 bool              `yaml:"enabled"`
		Port                    int               `yaml:"port"`
		Path                    string            `yaml:"path"`
		RecalcEstabConnsTimeout time.Duration     `yaml:"recalc_estab_timeout"`
		LiveRecalc              bool              `yaml:"live_recalc"`
		Labels                  map[string]string `yaml:"static_labels"`
		Detailed                bool              `yaml:"detailed"`
	} `yaml:"prometheus"`
	Radius struct {
		ListenAddr          string `yaml:"listen_addr"`
		Secret              string `yaml:"secret"`
		AgentParsingEnabled bool   `yaml:"agent_parsing_enabled"`
	} `yaml:"radius"`
	Api api.ApiConfig `yaml:"api"`

	Profiler struct {
		Port    int    `yaml:"port"`
		Path    string `yaml:"path"`
		Enabled bool   `yaml:"enabled"`
	} `yaml:"profiler"`
}

func LoadConfig(path string, Config *Configuration) error {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	yamlConfig := string(bytes)
	for _, e := range os.Environ() {
		pair := strings.SplitN(e, "=", 2)
		yamlConfig = strings.ReplaceAll(yamlConfig, fmt.Sprintf("${%v}", pair[0]), pair[1])
	}
	err = yaml.Unmarshal([]byte(yamlConfig), &Config)
	fmt.Printf(`Loaded configuration from %v with env readed:
%v
`, path, yamlConfig)
	if err != nil {
		return err
	}
	return nil
}

func ConfigureLogger(conf *Configuration) *logger.Logger {
	if conf.Logger.Console.Enabled {
		color := 0
		if conf.Logger.Console.EnableColor {
			color = 1
		}
		lg, _ := logger.New("radius", color, os.Stdout)
		lg.SetLogLevel(logger.LogLevel(conf.Logger.Console.LogLevel))
		if !conf.Logger.Console.PrintFile {
			lg.SetFormat("#%{id} %{time} > %{level} %{message}")
		} else {
			lg.SetFormat("#%{id} %{time} (%{filename}:%{line}) > %{level} %{message}")
		}
		return lg
	} else {
		lg, _ := logger.New("no_log", 0, os.DevNull)
		return lg
	}
}
