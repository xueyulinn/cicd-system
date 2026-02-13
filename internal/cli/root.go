package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "cicd",
	Short:         "CI/CD pipeline management tool",
	SilenceErrors: true,
	SilenceUsage:  true,
}

func init() {
	// register all subcommands
	rootCmd.AddCommand(verifyCmd)
	// rootCmd.AddCommand(reportCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(dryRunCmd)
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		// display suggested command help and usage when available
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		handleUnknownCommandError(err)
		os.Exit(1)
	}
}

// handles the unknown command error
func handleUnknownCommandError(err error) {
	unknownCmdError := "unknown command "
	// trigger the suggested command help and usage when the error is an unknown command error
	if strings.HasPrefix(err.Error(), unknownCmdError) {
		// get the incorrect subcmd input
		typedSubCmd := findTypedSubCmd(err.Error())
		if typedSubCmd == "" {
			os.Exit(1)
		}

		// get the suggestions derived by cobra
		suggestions := rootCmd.SuggestionsFor(typedSubCmd)

		// the suggestions could be empty if cobra cannot suggest a command
		if len(suggestions) > 0 {
			suggestedName := suggestions[0]

			// find the suggested command
			suggestedCmd := findCommandByName(rootCmd, suggestedName)

			if suggestedCmd != nil {
				// this will initialize the help flag for the suggested command
				suggestedCmd.InitDefaultHelpFlag()
				// print the help for the suggested command
				_ = suggestedCmd.Usage()
			}
		}
	}
}

// findTypedSubCmd extracts the typed subcommand from cobra's error message.
func findTypedSubCmd(errMsg string) string {
	// errMsg e.g. unknown command "veriff" for "cicd"
	const unknownPrefix = "unknown command "

	if !strings.HasPrefix(errMsg, unknownPrefix) {
		return ""
	}

	start := strings.Index(errMsg, "\"")
	if start == -1 {
		return ""
	}
	rest := errMsg[start+1:]
	end := strings.Index(rest, "\"")
	if end == -1 {
		return ""
	}

	typed := rest[:end]
	return typed
}

// findCommandByName finds a command by name in the root command
func findCommandByName(cmd *cobra.Command, cmdName string) *cobra.Command {
	// find the target subCmd
	if cmd.Name() == cmdName || cmd.HasAlias(cmdName) {
		return cmd
	}

	// recursively find the target subCmd
	for _, subCmd := range cmd.Commands() {
		if found := findCommandByName(subCmd, cmdName); found != nil {
			return found
		}
	}

	return nil
}
