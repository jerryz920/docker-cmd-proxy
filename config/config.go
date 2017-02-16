package config

import (
	"encoding/json"
	"log"
	"os"
	"path"
	"time"
)

const (
	CONFIG_FILE = "config.json"
)

type DaemonConfig struct {
	Timeout       time.Duration `json:"timeout,omitempty"`
	ContainerRoot string        `json:"container_root,omitempty"`
}

type MetadataServiceConfig struct {
	Protocol string `json omitempty`
	Address  string `json omitempty`
}

type TapconConfig struct {
	Daemon           DaemonConfig `json:"daemon,omitempty"`
	Metadata         MetadataServiceConfig
	StaticPortBase   int `json:"static_port_base,omitempty"`
	StaticPortMax    int `json:"static_port_max,omitempty"`
	PortPerContainer int `json:"port_per_container,omitempty"`
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
}
