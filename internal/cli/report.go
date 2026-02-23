package cli

import (
	"fmt"
	"strings"

	"github.com/CS7580-SEA-SP26/e-team/internal/models"
	"github.com/spf13/cobra"
)

var (
	reportPipeline string
	reportRun      int
	reportStage    string
	reportJob      string
)

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Report historical pipeline execution data",
	Long:  "Get reports on all runs for a pipeline, a specific run, stage, or job.",
	Args:  cobra.NoArgs,
	RunE:  runReport,
}

func init() {
	reportCmd.Flags().StringVar(&reportPipeline, "pipeline", "", "Pipeline name")
	reportCmd.Flags().IntVar(&reportRun, "run", 0, "Run number for the pipeline")
	reportCmd.Flags().StringVar(&reportStage, "stage", "", "Stage name (requires --run)")
	reportCmd.Flags().StringVar(&reportJob, "job", "", "Job name (requires --run and --stage)")
	reportCmd.Flags().StringP("format", "f", formatYAML, "Output format: yaml or json")
}

func runReport(cmd *cobra.Command, args []string) error {
	query, err := buildReportQuery()
	if err != nil {
		return err
	}

	format, err := cmd.Flags().GetString("format")
	if err != nil {
		return fmt.Errorf("failed to read format flag: %w", err)
	}
	format = strings.ToLower(strings.TrimSpace(format))
	if format != formatYAML && format != formatJSON {
		return fmt.Errorf("invalid format %q (supported: yaml, json)", format)
	}

	client := NewGatewayClient()
	report, err := client.Report(query)
	if err != nil {
		return err
	}

	var out []byte
	switch format {
	case formatJSON:
		out, err = FormatReportJSON(report)
	default:
		out, err = FormatReportYAML(report)
	}
	if err != nil {
		return err
	}

	fmt.Println(string(out))
	return nil
}

func buildReportQuery() (models.ReportQuery, error) {
	pipeline := strings.TrimSpace(reportPipeline)
	stage := strings.TrimSpace(reportStage)
	job := strings.TrimSpace(reportJob)

	if pipeline == "" {
		return models.ReportQuery{}, fmt.Errorf("pipeline is required (use --pipeline <name>)")
	}
	if reportRun < 0 {
		return models.ReportQuery{}, fmt.Errorf("run must be a positive integer")
	}
	if stage != "" && reportRun == 0 {
		return models.ReportQuery{}, fmt.Errorf("run is required when stage is provided")
	}
	if job != "" && (reportRun == 0 || stage == "") {
		return models.ReportQuery{}, fmt.Errorf("run and stage are required when job is provided")
	}

	query := models.ReportQuery{
		Pipeline: pipeline,
		Stage:    stage,
		Job:      job,
	}
	if reportRun > 0 {
		run := reportRun
		query.Run = &run
	}
	return query, nil
}
