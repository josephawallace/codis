package cmd

import (
	"codis/config"
	"codis/pkg/log"

	"github.com/spf13/cobra"
)

var (
	cfg    *config.Config
	logger *log.Logger
)

// NewRootCommand is the base command/name of the command line application.
func NewRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "codis",
		Short: "MPC application used to derive keys and create signatures for cryptocurrency transactions.",
		Long: `Codis is the cryptographic component of our cryptocurrency custody stack. Codis can distributively generate 
keys, and subsequently, sign transactions with those keys--also in a distributed manner. This ensures that the private 
key data never exists in it's entirety, and as such, is more resilient against attacks!`,
	}

	cfg = config.NewConfig()
	logger = log.NewLogger()

	cmd.AddCommand(startCmd())
	return cmd
}

func Execute() {
	cobra.CheckErr(NewRootCommand().Execute())
}
