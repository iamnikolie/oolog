package cmd

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

//go:embed skill.md
var skillDoc string

var skillCmd = &cobra.Command{
	Use:         "skill",
	Short:       "Print the embedded oolog reference",
	Annotations: map[string]string{"skipLoad": "true"},
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(os.Stdout, skillDoc)
		return nil
	},
}

func init() { rootCmd.AddCommand(skillCmd) }
