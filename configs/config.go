package configs

import (
	"codis/log"
	"github.com/spf13/cobra"
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

func NewConfig(cmd *cobra.Command) *Config {
	logger := log.NewLogger()

	configPath, _ := filepath.Abs("configs")
	viper.AddConfigPath(configPath)
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	envPrefix := "codis"
	viper.SetEnvPrefix(envPrefix)
	viper.AutomaticEnv()

	viper.SetDefault("ip", "0.0.0.0")
	viper.SetDefault("port", "0")
	viper.SetDefault("psk", "")
	viper.SetDefault("rendezvous", "")
	viper.SetDefault("client", "")
	viper.SetDefault("host", "")
	viper.SetDefault("bootstraps", "")

	if err := viper.BindPFlags(cmd.Flags()); err != nil {
		logger.Error(err)
	}
	if err := viper.BindPFlags(cmd.PersistentFlags()); err != nil {
		logger.Error(err)
	}

	if err := viper.ReadInConfig(); err != nil {
		logger.Error(err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		logger.Error(err)
	}

	return &cfg
}
