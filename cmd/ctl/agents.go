package main

import "github.com/spf13/cobra"

func newAgentsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "agents",
		Short: "List connected agents",
		RunE: func(cmd *cobra.Command, args []string) error {
			api := GetAPI(cmd.Context())
			return api.PrintGET("/agents")
		},
	}
}
