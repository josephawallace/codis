package cmd

import (
	"github.com/codenotary/immudb/embedded/store"
	"github.com/milquellc/codis/configs"
	"github.com/milquellc/codis/log"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	cfg    *configs.Config
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
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			db, err := filepath.Abs(".db")
			if err != nil {
				logger.Fatal(err)
			}
			if err = os.MkdirAll(db, store.DefaultFileMode); err != nil {
				logger.Fatal(err)
			}

			cfg = configs.NewConfig(cmd)
		},
	}

	logger = log.NewLogger()

	cmd.AddCommand(startCmd())
	return cmd
}

func Execute() {
	cobra.CheckErr(NewRootCommand().Execute())
}
