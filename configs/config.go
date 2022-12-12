package configs

import (
	"path/filepath"

	"github.com/milquellc/codis/log"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Config defines everything we need to know to run a node
type Config struct {
	IP         string   `mapstructure:"ip"`
	Port       string   `mapstructure:"port"`
	PSK        string   `mapstructure:"psk"`
	Rendezvous string   `mapstructure:"rendezvous"`
	Client     string   `mapstructure:"client"`
	Host       string   `mapstructure:"host"`
	Bootstraps []string `mapstructure:"bootstraps"`
	UseNodeKey bool     `mapstructure:"use_node_key"`
}

// NewConfig uses Viper to look through flags, environment variables, and config files to construct a config object that
// can be passed around when appropriate.
func NewConfig(cmd *cobra.Command) *Config {
	logger := log.NewLogger()

	configPath, _ := filepath.Abs(".")
	viper.AddConfigPath(configPath)
	viper.SetConfigName("codis-config")
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
	viper.SetDefault("use_node_key", false)

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
