package cmd

import (
	"codis/configs"

	"errors"

	"github.com/spf13/cobra"
)

var peerCfgId string

// startCmd is used to spin up a node. The type of node being created (bootstrap/peer/client) is indicated as a subcommand.
func startCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Starts a new node",
		Long:  `Creates a new node`,
		PersistentPreRunE: func(c *cobra.Command, args []string) error {
			if found := cfg.CheckPeerCfgExists(peerCfgId); found != true {
				return errors.New("no matching peer configuration found for id " + peerCfgId)
			}
			return nil
		},
	}

	cmd.PersistentFlags().StringVarP(&peerCfgId, "id", "i", configs.DefaultPeer.ID, "name of private key to use for peer identity")

	cmd.AddCommand(startPeerCmd())
	cmd.AddCommand(startBootstrapCmd())
	cmd.AddCommand(startClientCmd())

	return cmd
}
