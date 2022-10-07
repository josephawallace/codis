package config

import (
	"codis/pkg/log"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path/filepath"
)

type (
	Config struct {
		Peer    Peer    `yaml:"peer"`
		Network Network `yaml:"network"`
	}

	Peer struct {
		KeyID      string    `yaml:"keyId"`
		IP         string    `yaml:"ip"`
		Port       string    `yaml:"port"`
		Bootstraps []string  `yaml:"bootstraps"`
		Networks   []Network `yaml:"networks"`
	}

	Network struct {
		PSK string `yaml:"psk"`
	}
)

func NewConfig() *Config {
	logger := log.NewLogger()

	configFile, _ := filepath.Abs("config/config.yaml")
	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		logger.Fatal(err)
	}

	cfg := &Config{}
	err = yaml.Unmarshal(data, cfg)
	if err != nil {
		logger.Fatal(err)
	}

	if pskEnv := os.Getenv("PSK"); pskEnv != "" {
		cfg.Network.PSK = pskEnv
	}

	return cfg
}
