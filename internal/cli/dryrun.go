package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/CS7580-SEA-SP26/e-team/internal/common/parser"
	"github.com/CS7580-SEA-SP26/e-team/internal/common/planner"
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

	// Parse the configuration file
	p := parser.NewParser(configPath)
	pipeline, _, _ := p.Parse()

	// Generate execution plan (business logic)
	plan, err := planner.GenerateExecutionPlan(pipeline)
	if err != nil {
		return fmt.Errorf("failed to generate execution plan: %w", err)
	}

	// Require every stage to have at least one job (same as previous dryrun behaviour)
	for _, stage := range plan.Stages {
		if len(stage.Jobs) == 0 {
			return fmt.Errorf("stage '%s' has no jobs assigned to it", stage.Name)
		}
	}

	// Format output (presentation layer)
	format, _ := cmd.Flags().GetString("format")
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = formatYAML
	}
	var bytes []byte
	switch format {
	case formatYAML:
		bytes, err = FormatExecutionPlanYAML(plan)
	case formatJSON:
		bytes, err = FormatExecutionPlanJSON(plan)
	default:
		return fmt.Errorf("unsupported format %q (use yaml or json)", format)
	}
	if err != nil {
		return fmt.Errorf("failed to format output: %w", err)
	}
	fmt.Println(string(bytes))
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
