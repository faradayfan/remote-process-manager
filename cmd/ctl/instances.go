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
	cmd.AddCommand(newInstanceParamsCmd())
	cmd.AddCommand(newInstanceRenameCmd())

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

func newInstanceParamsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "params",
		Short: "Manage instance parameters",
	}

	cmd.AddCommand(newInstanceParamsSetCmd())
	cmd.AddCommand(newInstanceParamsUnsetCmd())
	return cmd
}

func newInstanceParamsSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <agentID> <instance> key=value [key=value ...]",
		Short: "Set or overwrite instance params (applies next start)",
		Args:  cobra.MinimumNArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			api := GetAPI(cmd.Context())

			agentID := url.PathEscape(args[0])
			inst := url.PathEscape(args[1])
			kvs := args[2:]

			payload := parseKeyValues(kvs)
			if len(payload) == 0 {
				return fmt.Errorf("no valid key=value pairs provided")
			}

			return api.PrintPOST(
				fmt.Sprintf("/agents/%s/instances/%s/params/set", agentID, inst),
				payload,
			)
		},
	}
}

func newInstanceParamsUnsetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unset <agentID> <instance> key [key ...]",
		Short: "Remove instance params (applies next start)",
		Args:  cobra.MinimumNArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			api := GetAPI(cmd.Context())

			agentID := url.PathEscape(args[0])
			inst := url.PathEscape(args[1])
			keys := args[2:]

			payload := map[string]any{
				"unset": keys,
			}

			return api.PrintPOST(
				fmt.Sprintf("/agents/%s/instances/%s/params/unset", agentID, inst),
				payload,
			)
		},
	}
}

func newInstanceRenameCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rename <agentID> <oldName> <newName>",
		Short: "Rename an instance (must be stopped)",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			api := GetAPI(cmd.Context())

			agentID := url.PathEscape(args[0])
			oldName := url.PathEscape(args[1])
			newName := args[2]

			payload := map[string]any{
				"new_name": newName,
			}

			return api.PrintPOST(
				fmt.Sprintf("/agents/%s/instances/%s/rename", agentID, oldName),
				payload,
			)
		},
	}
}
