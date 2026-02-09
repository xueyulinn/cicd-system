package cli

import (
	"fmt"

	"github.com/CS7580-SEA-SP26/e-team/internal/dryrun"
	"github.com/CS7580-SEA-SP26/e-team/internal/parser"
	"github.com/spf13/cobra"
)

var dryRunCmd = &cobra.Command{
	Use:   "dryrun [config-file]",
	Short: "Dry-run a pipeline configuration file",
	Long:  "Validate a pipeline configuration file and print the execution order for stages and jobs",
	Args:  cobra.MaximumNArgs(1),
	// validate the configuration file first
	PreRunE: runVerify,
	RunE: runDryRun,
}

func runDryRun (cmd *cobra.Command, args []string) error{
	// Get config path
	configPath := ".pipelines/pipeline.yaml"
	if len(args) > 0 {
		configPath = args[0]
	}

	// Parse the configuration file
	p := parser.NewParser(configPath)
	pipeline, _, _ := p.Parse()

	// Build the dry run output
	dryRunOutput := dryrun.BuildDryRunOutput(pipeline)
	// Marshal the dry run output
	bytes, err := dryrun.MarshalOutputStruct(dryRunOutput, pipeline.Stages)
	if err != nil {
		return fmt.Errorf("failed to marshal dry run output: %w", err)
	}
	fmt.Println(string(bytes))
	return nil
}