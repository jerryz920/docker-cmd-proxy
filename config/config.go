package config

import (
	"encoding/json"
	"os"
	"path"
	"time"

	log "github.com/Sirupsen/logrus"
)

const (
	CONFIG_FILE = "config.json"
)

type DaemonConfig struct {
	Timeout        time.Duration `json:"timeout,omitempty"`
	RefreshTimeout time.Duration `json:"refresh_timeout,omitempty"`
	ContainerRoot  string        `json:"container_root,omitempty"`
}

type MetadataServiceConfig struct {
	Protocol string `json omitempty`
	Address  string `json omitempty`
}

type TapconConfig struct {
	Daemon           DaemonConfig `json:"daemon,omitempty"`
	Metadata         MetadataServiceConfig
	StaticPortBase   int    `json:"static_port_base,omitempty"`
	StaticPortMax    int    `json:"static_port_max,omitempty"`
	PortPerContainer int    `json:"port_per_container,omitempty"`
	LogLevel         int    `json:"log_level,omitempty"`
	LogPath          string `json:"log_path,omitempty"`
}

const (
	DEFAULT_STATIC_PORT_BASE  = 15000
	DEFAULT_NUM_PER_CONTAINER = 100
	DEFAULT_STATIC_PORT_MAX   = 35000
)

var Config *TapconConfig

func InitConf(config_path string) {
	conf_file := path.Join(config_path, CONFIG_FILE)
	f, err := os.Open(conf_file)
	if err != nil {
		log.Fatalf("error in reading configuration file %v", err)
	}
	Config = &TapconConfig{}
	decoder := json.NewDecoder(f)
	if err := decoder.Decode(Config); err != nil {
		log.Fatalf("error in decoding the configuration file %v", err)
	}

	if Config.StaticPortBase == 0 {
		Config.StaticPortBase = DEFAULT_STATIC_PORT_BASE
	}
	if Config.StaticPortMax == 0 {
		Config.StaticPortMax = DEFAULT_STATIC_PORT_MAX
	}
	if Config.PortPerContainer == 0 {
		Config.PortPerContainer = DEFAULT_NUM_PER_CONTAINER
	}
	if Config.LogLevel == 0 {
		log.SetLevel(log.DebugLevel)
	} else if Config.LogLevel == 1 {
		log.SetLevel(log.InfoLevel)
	} else if Config.LogLevel == 2 {
		log.SetLevel(log.WarnLevel)
	} else {
		log.SetLevel(log.ErrorLevel)
	}

	if Config.LogPath == "" {
		log.SetOutput(os.Stdout)
	} else {
		f, err := os.OpenFile(Config.LogPath, os.O_APPEND, 0644)
		if err != nil {
			log.Fatalf("can not open log path %s to write\n",
				Config.LogPath)
		}
		log.SetOutput(f)
	}
}
