package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type RootOptions struct {
	BaseURL   string
	APIPrefix string
	Timeout   time.Duration
}

func Execute(args []string) {
	root := newRootCmd()
	root.SetArgs(args)

	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	opts := &RootOptions{
		BaseURL:   getenvDefault("GAMESVC_URL", "http://127.0.0.1:8080"),
		APIPrefix: "/v1",
		Timeout:   10 * time.Second,
	}

	cmd := &cobra.Command{
		Use:   "gamesvcctl",
		Short: "CLI for remote-process-manager control plane",
		Long:  "gamesvcctl controls game server instances via the command-server HTTP API.",
	}

	cmd.PersistentFlags().StringVar(&opts.BaseURL, "url", opts.BaseURL, "Command-server base URL (or set GAMESVC_URL)")
	cmd.PersistentFlags().DurationVar(&opts.Timeout, "timeout", opts.Timeout, "HTTP request timeout")
	cmd.PersistentFlags().StringVar(&opts.APIPrefix, "api-prefix", opts.APIPrefix, "API prefix (default: /v1)")

	// Attach shared dependencies to context
	cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		opts.BaseURL = strings.TrimRight(opts.BaseURL, "/")

		httpClient := &http.Client{Timeout: opts.Timeout}
		api := NewAPI(httpClient, opts.BaseURL, opts.APIPrefix)

		cmd.SetContext(WithAPI(cmd.Context(), api))
		return nil
	}

	// Register subcommands
	cmd.AddCommand(newAgentsCmd())
	cmd.AddCommand(newInstancesCmd())
	cmd.AddCommand(newTemplatesCmd())

	return cmd
}
