package cmd

import "testing"

func TestRootCommandName(t *testing.T) {
	if rootCmd.Use != "oolog" {
		t.Fatalf("rootCmd.Use = %q, want %q", rootCmd.Use, "oolog")
	}
}
