package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xueyulinn/cicd-system/internal/common/formatter"
	"github.com/xueyulinn/cicd-system/internal/models"
)

var (
	reportRun   int
	reportStage string
	reportJob   string
)

var reportCmd = &cobra.Command{
	Use:   "report pipeline-name [--run run-no] [--stage stage-name] [--job job-name] [-f yaml|json]",
	Short: "Show pipeline run reports",
	Long:  "Query pipeline report data for a specific run, and optionally narrow it to one stage or one job.",
	Example: strings.Join([]string{
		"  cicd report DefaultPipeline --run 1",
		"  cicd report DefaultPipeline --run 1 --stage build",
		"  cicd report DefaultPipeline --run 1 --stage build --job compile",
		"  cicd report DefaultPipeline --run 1 -f json",
	}, "\n"),
	Args:                  cobra.ExactArgs(1),
	RunE:                  runReport,
	DisableFlagsInUseLine: true,
}

func init() {
	reportCmd.Flags().IntVar(&reportRun, "run", 0, "Pipeline run number (optional)")
	reportCmd.Flags().StringVar(&reportStage, "stage", "", "Stage filter (requires --run)")
	reportCmd.Flags().StringVar(&reportJob, "job", "", "Job filter (requires --run and --stage)")
	reportCmd.Flags().StringP("format", "f", formatYAML, "Output format (yaml|json)")
}

func runReport(cmd *cobra.Command, args []string) error {
	query, err := buildReportQuery(args)
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

	reportOutput, err := formatReport(report, format)
	if err != nil {
		return err
	}
	fmt.Println(string(reportOutput))
	return nil
}

func formatReport(report *models.ReportResponse, format string) ([]byte, error) {
	var out []byte
	var err error
	switch format {
	case formatJSON:
		out, err = formatter.FormatReportJSON(report)
	default:
		out, err = formatter.FormatReportYAML(report)
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}

func buildReportQuery(args []string) (models.ReportQuery, error) {
	pipeline := args[0]
	stage := strings.TrimSpace(reportStage)
	job := strings.TrimSpace(reportJob)

	if pipeline == "" {
		return models.ReportQuery{}, fmt.Errorf("pipeline name is required")
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
		runNo := reportRun
		query.Run = &runNo
	}
	return query, nil
}
