package cmd

import (
	"github.com/spf13/cobra"
)

// startCmd is used to spin up a node. The type of node being created (bootstrap/peer) is indicated as a subcommand.
func startCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Starts a new node as either a bootstrapping node or peer",
		Long: `Creates a new node. The node could be a bootstrapping node, in which case is responsible for helping
peer discovery. Otherwise, the node can be a peer, in which case it connects to a given bootstrap node when starting up.`,
	}

	cmd.AddCommand(startPeerCmd())
	cmd.AddCommand(startBootstrapCmd())
	cmd.AddCommand(startClientCmd())

	return cmd
}
