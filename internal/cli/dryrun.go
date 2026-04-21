package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xueyulinn/cicd-system/internal/common/gitutil"
	"github.com/xueyulinn/cicd-system/internal/common/parser"
	"github.com/xueyulinn/cicd-system/internal/common/planner"
	"github.com/xueyulinn/cicd-system/internal/common/verifier"
)

const (
	formatYAML = "yaml"
	formatJSON = "json"
)

var dryRunCmd = &cobra.Command{
	Use:   "dryrun pipeline-path",
	Short: "Dryrun a pipeline file",
	Long:  "Validate a pipeline file first then outputs the execution order for jobs",
	Args:  cobra.ExactArgs(1),
	// validate the configuration file first
	PreRunE: runVerify,
	RunE:    runDryRun,
}

func init() {
	dryRunCmd.Flags().StringP("format", "f", formatYAML, "Output format: yaml or json")
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
	response, err := client.DryRun(string(fileContent))
	if err != nil {
		return fmt.Errorf("dry run failed, error: %w", err)
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

// runDryRunDirect performs dry run without gateway (for testing)
func runDryrunDirect(configPath, yamlContent string) error {
	// Create a temporary file for parsing
	tmpFile, err := os.CreateTemp("", "test-*.yaml")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	if _, err := tmpFile.WriteString(yamlContent); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	p := parser.NewParser(tmpFile.Name())
	pipeline, rootNode, err := p.Parse()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", configPath, err.Error())
		return err
	}

	v := verifier.NewPipelineVerifier(tmpFile.Name(), pipeline, rootNode)
	errors := v.Verify()
	if len(errors) > 0 {
		for _, err := range errors {
			fmt.Fprintln(os.Stderr, err.Error())
		}
		// Return the first error directly for test compatibility
		return errors[0]
	}

	// For test mode, generate proper dry run output
	plan, err := planner.GenerateExecutionPlan(pipeline)
	if err != nil {
		return fmt.Errorf("failed to generate execution plan: %w", err)
	}

	// Generate dry run output in the expected format (stage: {job: {...}})
	// Build the output as a string to preserve order
	var output strings.Builder
	
	for _, stage := range plan.Stages {
		output.WriteString(fmt.Sprintf("%s:\n", stage.Name))
		for _, job := range stage.Jobs {
			output.WriteString(fmt.Sprintf("    %s:\n", job.Name))
			output.WriteString(fmt.Sprintf("        image: %s\n", job.Image))
			output.WriteString("        script:\n")
			for _, scriptLine := range job.Script {
				output.WriteString(fmt.Sprintf("            - %s\n", scriptLine))
			}
		}
	}
	
	fmt.Print(output.String())
	return nil
}
