package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xueyulinn/cicd-system/internal/api"
	"github.com/xueyulinn/cicd-system/internal/common/formatter"
	"github.com/xueyulinn/cicd-system/internal/common/gitutil"
	"github.com/xueyulinn/cicd-system/internal/models"
)

const (
	formatYAML = "yaml"
	formatJSON = "json"
)

var formatting string

var dryRunCmd = &cobra.Command{
	Use:                   "dryrun pipeline-path [--format yaml|json]",
	Short:                 "Validate a pipeline and preview its execution plan",
	Long:                  "Validates the specified pipeline file and prints the execution plan without running any jobs. Output defaults to YAML and can be changed with --format json.",
	Example:               "cicd dryrun .pipelines/pipeline.yaml\ncicd dryrun .pipelines/pipeline.yaml --format json",
	Args:                  cobra.ExactArgs(1),
	PreRunE:               runValidateQuiet,
	RunE:                  runDryRun,
	DisableFlagsInUseLine: true,
}

func init() {
	dryRunCmd.Flags().StringVarP(&formatting, "format", "f", formatYAML, "Execution plan output format: yaml or json")
}

func runDryRun(cmd *cobra.Command, args []string) error {
	repo, err := gitutil.Open(".")
	if err != nil {
		return err
	}
	rootDir := repo.Root()
	pipelinePath := args[0]

	completePath := pipelinePath
	if !filepath.IsAbs(pipelinePath) {
		completePath = filepath.Join(rootDir, pipelinePath)
	}
	completePath = filepath.Clean(completePath)

	// Read file content
	fileContent, err := os.ReadFile(completePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Create gateway client
	client := NewGatewayClient()

	// Call gateway for dry run
	response, err := client.DryRun(api.ValidateRequest{YAMLContent: string(fileContent)})
	if err != nil {
		return fmt.Errorf("dry run failed, error: %w", err)
	}

	if !response.Valid {
		for _, errMsg := range response.Errors {
			fmt.Fprintln(os.Stderr, errMsg)
		}
		for _, errMsg := range response.Errors {
			msg := strings.TrimSpace(errMsg)
			if msg != "" {
				return fmt.Errorf("dry run failed with %d error(s): %s", len(response.Errors), msg)
			}
		}
		return fmt.Errorf("dry run failed with %d error(s)", len(response.Errors))
	}

	// Print dryrun output
	output, err := formatOutput(response.ExecutionPlan, formatting)
	if err != nil {
		return err
	}
	fmt.Println(output)
	return nil
}

func formatOutput(plan *models.ExecutionPlan, format string) (string, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = formatYAML
	}

	var (
		out []byte
		err error
	)
	switch format {
	case formatYAML:
		out, err = formatter.FormatExecutionPlanYAML(plan)
	case formatJSON:
		out, err = formatter.FormatExecutionPlanJSON(plan)
	default:
		return "", fmt.Errorf("invalid format %q (supported: yaml, json)", format)
	}
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// runValidateQuiet runs the validate command but redirects stdout to /dev/null.
func runValidateQuiet(cmd *cobra.Command, args []string) error {
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		return err
	}
	defer func() { _ = devNull.Close() }()
	stdout := os.Stdout
	os.Stdout = devNull
	defer func() { os.Stdout = stdout }()
	return runValidate(cmd, args)
}
