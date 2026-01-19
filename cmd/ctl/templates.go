package main

import (
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
)

func newTemplatesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "templates",
		Short: "Manage server templates",
	}

	cmd.AddCommand(newTemplatesListCmd())
	cmd.AddCommand(newTemplatesInspectCmd())

	return cmd
}

func newTemplatesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <agentID>",
		Short: "List templates available on an agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			api := GetAPI(cmd.Context())
			agentID := url.PathEscape(args[0])
			return api.PrintGET(fmt.Sprintf("/agents/%s/templates", agentID))
		},
	}
}

func newTemplatesInspectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "inspect <agentID> <templateName>",
		Short: "Inspect one template on an agent",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			api := GetAPI(cmd.Context())
			agentID := url.PathEscape(args[0])
			name := url.PathEscape(args[1])
			return api.PrintGET(fmt.Sprintf("/agents/%s/templates/%s", agentID, name))
		},
	}
}
