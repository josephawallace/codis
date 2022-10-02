package configs

import "os"

type Config struct {
	BootstrapAddrs []string
	Rendezvous     string
	PSK            string
}

func New() Config {
	return Config{
		BootstrapAddrs: []string{},
		Rendezvous:     os.Getenv("RENDEZVOUS"),
		PSK:            os.Getenv("PSK"),
	}
}
