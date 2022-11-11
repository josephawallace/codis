package configs

import (
	"codis/log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

type (
	Config struct {
		Peers map[string]Peer `yaml:"peers"`
	}

	Peer struct {
		ID         string
		IP         string   `yaml:"ip"`
		Port       string   `yaml:"port"`
		Host       string   `yaml:"host,omitempty"`
		Bootstraps []string `yaml:"bootstraps,omitempty"`
		Network    Network  `yaml:"networks"`
	}

	Network struct {
		PSK string `yaml:"psk"`
	}
)

var (
	DefaultNetwork = Network{
		PSK: "",
	}

	DefaultPeer = Peer{
		ID:         "",
		IP:         "0.0.0.0",
		Port:       "0",
		Bootstraps: []string{},
		Network:    DefaultNetwork,
	}
)

func NewConfig() *Config {
	logger := log.NewLogger()

	configFile, _ := filepath.Abs("configs/config.yaml")
	data, err := os.ReadFile(configFile)
	if err != nil {
		logger.Fatal(err)
	}

	cfg := &Config{}
	err = yaml.Unmarshal(data, cfg)
	if err != nil {
		logger.Fatal(err)
	}

	cfg.Peers[DefaultPeer.ID] = DefaultPeer

	for key, _ := range cfg.Peers {
		if peerCfg, ok := cfg.Peers[key]; ok {
			peerCfg.ID = key
			cfg.Peers[key] = peerCfg
		}
	}

	return cfg
}

func (c *Config) CheckPeerCfgExists(id string) bool {
	for _, peerCfg := range c.Peers {
		if id == peerCfg.ID {
			return true
		}
	}
	return false
}
