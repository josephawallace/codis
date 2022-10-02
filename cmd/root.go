package cmd

import (
	"codis/configs"
	"github.com/spf13/cobra"
)

var config = configs.New()

// NewRootCommand is the base command/name of the command line application.
func NewRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "codis",
		Short: "MPC application used to derive keys and create signatures for cryptocurrency transactions.",
		Long: `Codis is the cryptography component of our cryptocurrency custody stack. Codis can distributively generate 
keys, and subsequently, sign transactions with those keys also in a distributed manner. This ensures that the full
private key data never exists in one place, so is more resilient against attack!`,
	}

	cmd.AddCommand(startCmd())

	return cmd
}

func Execute() {
	cobra.CheckErr(NewRootCommand().Execute())
}
