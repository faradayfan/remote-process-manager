package main

import (
	"fmt"
	"net/url"

	"github.com/faradayfan/remote-process-manager/internal/protocol"
	"github.com/spf13/cobra"
)

func newInstancesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "instances",
		Short: "Manage server instances",
	}

	cmd.AddCommand(newInstanceCreateCmd())
	cmd.AddCommand(newInstanceDeleteCmd())
	cmd.AddCommand(newInstanceDisableCmd())
	cmd.AddCommand(newInstanceEnableCmd())
	cmd.AddCommand(newInstancesListCmd())
	cmd.AddCommand(newInstanceStartCmd())
	cmd.AddCommand(newInstanceStatusCmd())
	cmd.AddCommand(newInstanceStopCmd())

	return cmd
}

func newInstancesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <agentID>",
		Short: "List instances on an agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			api := GetAPI(cmd.Context())
			agentID := url.PathEscape(args[0])
			return api.PrintGET(fmt.Sprintf("/agents/%s/instances", agentID))
		},
	}
}

func newInstanceCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create <agentID> <name> <template> [key=value ...]",
		Short: "Create an instance",
		Args:  cobra.MinimumNArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			api := GetAPI(cmd.Context())

			agentID := args[0]
			name := args[1]
			template := args[2]
			params := parseKeyValues(args[3:])

			req := protocol.CreateInstanceRequest{
				Name:     name,
				Template: template,
				Enabled:  true,
				Params:   params,
			}

			return api.PrintPOST(
				fmt.Sprintf("/agents/%s/instances/create", url.PathEscape(agentID)),
				req,
			)
		},
	}
}

func newInstanceDeleteCmd() *cobra.Command {
	var force bool
	var deleteData bool

	cmd := &cobra.Command{
		Use:   "delete <agentID> <name>",
		Short: "Delete an instance",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			api := GetAPI(cmd.Context())

			agentID := args[0]
			name := args[1]

			req := protocol.DeleteInstanceRequest{
				Name:       name,
				Force:      force,
				DeleteData: deleteData,
			}

			return api.PrintPOST(
				fmt.Sprintf("/agents/%s/instances/delete", url.PathEscape(agentID)),
				req,
			)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Force delete (stop running process if needed)")
	cmd.Flags().BoolVar(&deleteData, "delete-data", false, "Delete instance data directory")

	return cmd
}

func newInstanceStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start <agentID> <instance>",
		Short: "Start an instance",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			api := GetAPI(cmd.Context())
			agentID := url.PathEscape(args[0])
			instance := url.PathEscape(args[1])
			return api.PrintPOST(fmt.Sprintf("/agents/%s/instances/%s/start", agentID, instance), nil)
		},
	}
}

func newInstanceStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop <agentID> <instance>",
		Short: "Stop an instance",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			api := GetAPI(cmd.Context())
			agentID := url.PathEscape(args[0])
			instance := url.PathEscape(args[1])
			return api.PrintPOST(fmt.Sprintf("/agents/%s/instances/%s/stop", agentID, instance), nil)
		},
	}
}

func newInstanceStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <agentID> <instance>",
		Short: "Get instance status",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			api := GetAPI(cmd.Context())
			agentID := url.PathEscape(args[0])
			instance := url.PathEscape(args[1])
			return api.PrintGET(fmt.Sprintf("/agents/%s/instances/%s/status", agentID, instance))
		},
	}
}

func newInstanceEnableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "enable <agentID> <instance>",
		Short: "Enable an instance (allow starting)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			api := GetAPI(cmd.Context())
			agentID := url.PathEscape(args[0])
			instance := url.PathEscape(args[1])
			return api.PrintPOST(fmt.Sprintf("/agents/%s/instances/%s/enable", agentID, instance), map[string]any{})
		},
	}
}

func newInstanceDisableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "disable <agentID> <instance>",
		Short: "Disable an instance (prevent starting)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			api := GetAPI(cmd.Context())
			agentID := url.PathEscape(args[0])
			instance := url.PathEscape(args[1])
			return api.PrintPOST(fmt.Sprintf("/agents/%s/instances/%s/disable", agentID, instance), map[string]any{})
		},
	}
}
