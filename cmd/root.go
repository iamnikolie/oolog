// Package cmd wires the oolog cobra command tree.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/iamnikolie/oolog/internal/cache"
	"github.com/iamnikolie/oolog/internal/client"
	"github.com/iamnikolie/oolog/internal/config"
)

// Package-level state populated by PersistentPreRunE.
var (
	account   string // --config value or OOLOG_CONFIG env; "" = default account
	outFormat string // --format: json|csv|tsv|"" (default = fallback printer)
	outJSON   bool   // --json shortcut for --format json
	verbose   bool   // --verbose: dump HTTP to stderr
)

var (
	cli     *client.Client
	cfg     config.Config
	streams cache.Streams
)

var rootCmd = &cobra.Command{
	Use:           "oolog",
	Short:         "Agent-friendly CLI for querying OpenObserve logs",
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&account, "config", "", "account name (multi-account); default account if empty")
	rootCmd.PersistentFlags().StringVar(&outFormat, "format", "", "output format: json|csv|tsv (default: human/LLM-friendly)")
	rootCmd.PersistentFlags().BoolVar(&outJSON, "json", false, "shortcut for --format json")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "print HTTP request/response to stderr")

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		// Commands that must run without a config skip loading.
		if cmd.Annotations["skipLoad"] == "true" {
			return nil
		}
		return ensureLoaded(cmd)
	}
}

// ensureLoaded populates cfg, cli, and the streams cache (auto-fetching it once).
func ensureLoaded(cmd *cobra.Command) error {
	var err error
	cfg, err = config.Load(account)
	if err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	if err := cfg.ResolvePassword(); err != nil {
		return err
	}
	cli = client.New(cfg, verbose)
	streams, err = cache.Load(account)
	if err != nil {
		// First run: fetch and persist the streams schema.
		streams, err = cli.Streams(cmd.Context())
		if err != nil {
			return fmt.Errorf("fetch streams: %w", err)
		}
		if err := cache.Save(streams, account); err != nil {
			return fmt.Errorf("save streams cache: %w", err)
		}
	}
	return nil
}

// Execute runs the root command and returns a process exit code.
func Execute() int {
	if account == "" {
		account = os.Getenv("OOLOG_CONFIG")
	}
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	return 0
}
