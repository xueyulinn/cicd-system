package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "cicd",
	Short: "A local-first CI/CD tool for validating and simulating pipelines",
	Long: `cicd is a local-first CI/CD tool designed to help developers validate,
simulate, and reason about CI/CD pipelines before pushing changes to a remote
CI system.

It allows users to verify pipeline configurations, inspect execution order,
and simulate pipeline runs locally without modifying repository configuration
or relying on shared CI infrastructure.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Welcome to the CI/CD tool. Use the -h flag to see available commands.")
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Oops. An error while executing Zero '%s'\n", err)
		os.Exit(1)
	}
}
