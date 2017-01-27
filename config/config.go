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
	Timeout time.Duration `json omitempty`
}

type TapconConfig struct {
	Daemon DaemonConfig
}

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
}
