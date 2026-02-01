package main

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "cicd",
	Short: "CI/CD pipeline management tool",
}

func init() {
	// register all subcommands
	rootCmd.AddCommand(verifyCmd)
	// rootCmd.AddCommand(reportCmd)
	// rootCmd.AddCommand(runCmd)      // 未来添加
	// rootCmd.AddCommand(dryRunCmd)   // 未来添加
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
