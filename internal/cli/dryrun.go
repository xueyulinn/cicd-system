package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

const (
	formatYAML = "yaml"
	formatJSON = "json"
)

var dryRunCmd = &cobra.Command{
	Use:   "dryrun [config-file]",
	Short: "Dry-run a pipeline configuration file",
	Long:  "Validate a pipeline configuration file and print the execution order for stages and jobs",
	Args:  cobra.MaximumNArgs(1),
	// validate the configuration file first
	PreRunE: runVerifyQuiet,
	RunE:    runDryRun,
}

func init() {
	dryRunCmd.Flags().StringP("format", "f", formatYAML, "Output format: yaml or json")
}

func runDryRun(cmd *cobra.Command, args []string) error {
	// Get config path
	configPath := ".pipelines/pipeline.yaml"
	if len(args) > 0 {
		configPath = args[0]
	}

	// Read file content
	fileContent, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Create gateway client
	client := NewGatewayClient()

	// Call gateway for dry run
	response, err := client.DryRun(string(fileContent))
	if err != nil {
		// Extract just the validation error message without file path
		errorMsg := err.Error()
		if strings.Contains(errorMsg, "gateway returned status") {
			// Look for the actual validation error message
			start := strings.Index(errorMsg, "content:")
			if start != -1 {
				errorMsg = errorMsg[start+8:] // Skip "content:" prefix
				// Remove any trailing JSON artifacts more thoroughly
				errorMsg = strings.TrimSuffix(errorMsg, "\"}")
				errorMsg = strings.TrimSuffix(errorMsg, "\"")
				errorMsg = strings.TrimSuffix(errorMsg, "}")
			}
		}
		// Fix Unicode escaping
		errorMsg = strings.ReplaceAll(errorMsg, "\\u003e", ">")
		fmt.Fprintf(os.Stderr, "%s: %s\n", configPath, errorMsg)
		return fmt.Errorf("dry run failed with %d error(s)", 1)
	}

	if !response.Valid {
		for _, errMsg := range response.Errors {
			fmt.Fprintln(os.Stderr, errMsg)
		}
		return fmt.Errorf("dry run failed with %d error(s)", len(response.Errors))
	}

	// Print dry run output
	fmt.Println(response.Output)
	return nil
}

// runVerifyQuiet runs the verify command but redirects stdout to /dev/null
func runVerifyQuiet(cmd *cobra.Command, args []string) error {
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		return err
	}
	defer func() { _ = devNull.Close() }()
	stdout := os.Stdout
	os.Stdout = devNull
	defer func() { os.Stdout = stdout }()
	return runVerify(cmd, args)
}
