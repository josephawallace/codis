package cmd

import (
	"errors"

	"codis/config"

	"github.com/spf13/cobra"
)

var peerCfgId string

// startCmd is used to spin up a node. The type of node being created (bootstrap/peer) is indicated as a subcommand.
func startCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Starts a new node as either a bootstrapping node or peer",
		Long: `Creates a new node. The node could be a bootstrapping node, in which case is responsible for helping
peer discovery. Otherwise, the node can be a peer, in which case it connects to a given bootstrap node when starting up.`,
		PersistentPreRunE: func(c *cobra.Command, args []string) error {
			if found := cfg.CheckPeerCfgExists(peerCfgId); found != true {
				return errors.New("no matching peer configuration found for id " + peerCfgId)
			}
			return nil
		},
	}

	cmd.PersistentFlags().StringVarP(&peerCfgId, "id", "i", config.DefaultPeer.ID, "name of private key to use for peer identity")

	cmd.AddCommand(startPeerCmd())
	cmd.AddCommand(startBootstrapCmd())
	cmd.AddCommand(startClientCmd())

	return cmd
}
