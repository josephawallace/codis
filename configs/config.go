package configs

import (
	"codis/log"
	"github.com/spf13/viper"
	"path/filepath"
)

type (
	Config struct {
		IP         string   `mapstructure:"ip"`
		Port       string   `mapstructure:"port"`
		PSK        string   `mapstructure:"psk"`
		Rendezvous string   `mapstructure:"rendezvous"`
		Client     string   `mapstructure:"client"`
		Host       string   `mapstructure:"host"`
		Bootstraps []string `mapstructure:"bootstraps"`
	}
)

func NewConfig() *Config {
	logger := log.NewLogger()

	viper.SetConfigName("app")
	viper.SetConfigType("env")

	envPrefix := "codis"
	viper.SetEnvPrefix(envPrefix)
	viper.AutomaticEnv() // get CODIS_ENV vars like viper.Get("env")

	configDirPath, _ := filepath.Abs("env/")
	viper.AddConfigPath(configDirPath)

	for _, key := range []string{
		"ip",
		"port",
		"psk",
		"rendezvous",
		"client",
		"host",
		"bootstraps",
	} {
		if err := viper.BindEnv(key); err != nil {
			logger.Error(err)
		}
	}

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			logger.Debug("config file not found, using defaults and environment variables")
		} else {
			logger.Error(err)
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		logger.Error(err)
	}

	// HOTFIX: unmarshalling "[]" from .env gives an array of length one with ""

	return &cfg
}
