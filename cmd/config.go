package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/iamnikolie/oolog/internal/cache"
	"github.com/iamnikolie/oolog/internal/client"
	"github.com/iamnikolie/oolog/internal/config"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	cfgURL, cfgOrg, cfgEmail, cfgPassword, cfgStream, cfgPasswordCmd string
)

var configCmd = &cobra.Command{
	Use:         "config",
	Short:       "Manage oolog account configuration",
	Annotations: map[string]string{"skipLoad": "true"},
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create ~/.oolog/config.yaml for an account",
	Long: `Create the config for an account after validating it by listing streams.

Password: --password, else $OOLOG_PASSWORD (validated but NOT written to the
file), else --password-command (stored as a command, not a secret), else a
prompt. SSH tunnels and field mappings are added to the YAML by hand
(see 'oolog skill').`,
	Annotations: map[string]string{"skipLoad": "true"},
	RunE: func(cmd *cobra.Command, args []string) error {
		r := bufio.NewReader(os.Stdin)
		url := prompt(r, "OpenObserve URL", cfgURL, "http://localhost:5080")
		org := prompt(r, "Org", cfgOrg, "default")
		email := prompt(r, "Email", cfgEmail, "")
		c := config.Config{URL: url, Org: org, Email: email, PasswordCommand: cfgPasswordCmd}

		switch {
		case cfgPassword != "":
			c.Password = cfgPassword
		case os.Getenv("OOLOG_PASSWORD") != "" || cfgPasswordCmd != "":
			// Resolved for validation only; never persisted.
		default:
			fmt.Fprint(os.Stderr, "Password: ")
			b, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Fprintln(os.Stderr)
			if err != nil {
				return err
			}
			c.Password = strings.TrimSpace(string(b))
		}

		// Validate by listing streams before writing.
		vc := c
		if err := vc.ResolvePassword(); err != nil {
			return err
		}
		list, err := client.New(vc, verbose).Streams(cmd.Context())
		if err != nil {
			return fmt.Errorf("validation failed (check URL/credentials): %w", err)
		}
		c.Stream = prompt(r, "Default stream (optional, "+streamHint(list.Names())+")", cfgStream, "")
		if err := config.Save(c, account); err != nil {
			return err
		}
		if err := cache.Save(list, account); err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "saved config for account: %s (%d streams)\n", accountName(), len(list.List))
		return nil
	},
}

func streamHint(names []string) string {
	if len(names) == 0 {
		return "none yet"
	}
	return "found: " + strings.Join(names, ", ")
}

func accountName() string {
	if account == "" {
		return "default"
	}
	return account
}

func prompt(r *bufio.Reader, label, flagVal, def string) string {
	if flagVal != "" {
		return flagVal
	}
	if def != "" {
		fmt.Fprintf(os.Stderr, "%s [%s]: ", label, def)
	} else {
		fmt.Fprintf(os.Stderr, "%s: ", label)
	}
	line, _ := r.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	return line
}

func init() {
	configInitCmd.Flags().StringVar(&cfgURL, "url", "", "OpenObserve base URL")
	configInitCmd.Flags().StringVar(&cfgOrg, "org", "", "organization (default: default)")
	configInitCmd.Flags().StringVar(&cfgEmail, "email", "", "login email")
	configInitCmd.Flags().StringVar(&cfgPassword, "password", "", "login password (omit to use $OOLOG_PASSWORD or prompt)")
	configInitCmd.Flags().StringVar(&cfgPasswordCmd, "password-command", "", "shell command whose stdout is the password (stored as a command)")
	configInitCmd.Flags().StringVar(&cfgStream, "stream", "", "default stream (optional)")
	configCmd.AddCommand(configInitCmd)
	rootCmd.AddCommand(configCmd)
}
