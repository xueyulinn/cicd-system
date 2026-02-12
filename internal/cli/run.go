package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Execute a pipeline locally",
	Long:  "Run a pipeline with the given name or file. For the initial iteration, all pipeline executions happen locally.",
	Args:  cobra.NoArgs,
	RunE:  runRun,
}

func runRun(cmd *cobra.Command, args []string) error {
	// TODO: implement cicd run (--name, --file, --branch, --commit)
	return fmt.Errorf("run: not implemented yet")
}
