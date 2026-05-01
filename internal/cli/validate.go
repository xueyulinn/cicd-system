package cli

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xueyulinn/cicd-system/internal/api"
	"github.com/xueyulinn/cicd-system/internal/common/gitutil"
)

var validateCmd = &cobra.Command{
	Use:                   "validate {pipeline-path|pipeline-directory}",
	Short:                 "Validate a pipeline file or directory",
	Long:                  "Validate a single pipeline YAML file, or recursively validate all pipeline YAML files under a directory. The command stops at the first failure and prints the exact error location.",
	Example:               "cicd validate ./.pipelines/build.yaml\ncicd validate ./.pipelines",
	Args:                  cobra.ExactArgs(1),
	RunE:                  runValidate,
	Aliases:               []string{"verify"},
	DisableFlagsInUseLine: true,
}

func runValidate(cmd *cobra.Command, args []string) error {
	repo, ok := cmd.Context().Value(repoKey).(*gitutil.Repository)
	if !ok || repo == nil {
		return fmt.Errorf("git repository context is missing")
	}
	rootDir := repo.Root()

	pipelinePath := args[0]
	completePath := pipelinePath
	if !filepath.IsAbs(pipelinePath) {
		completePath = filepath.Join(rootDir, pipelinePath)
	}
	completePath = filepath.Clean(completePath)

	info, err := os.Stat(completePath)
	if err != nil {
		return fmt.Errorf("failed to get the info of path %q: %w", completePath, err)
	}

	gatewayClient := NewGatewayClient()
	if info.IsDir() {
		return validatePipelineDir(completePath, gatewayClient)
	}

	valid, err := validateSinglePipeline(completePath, gatewayClient)
	if err != nil {
		return err
	}
	if valid {
		fmt.Println("pipeline is valid")
	}

	return nil
}

func validatePipelineDir(dir string, gatewayClient *GatewayClient) error {
	targets, err := collectYAMLFiles(dir)
	if err != nil {
		return fmt.Errorf("failed to enumerate directory %q: %w", dir, err)
	}
	if len(targets) == 0 {
		return fmt.Errorf("no YAML files found in directory: %s", dir)
	}

	sort.Strings(targets)

	for _, target := range targets {
		valid, err := validateSinglePipeline(target, gatewayClient)
		if err != nil {
			// Fast fail: stop at the first invalid file or validation request error.
			return err
		}

		if valid {
			fmt.Printf("%s: Configuration is valid\n", target)
		}
	}

	return nil
}

func validateSinglePipeline(pipelinePath string, gatewayClient *GatewayClient) (bool, error) {
	fileContent, err := os.ReadFile(pipelinePath)
	if err != nil {
		return false, fmt.Errorf("failed to read file content %q: %w", pipelinePath, err)
	}

	response, err := gatewayClient.Validate(api.ValidateRequest{YAMLContent: string(fileContent)})
	if err != nil {
		return false, fmt.Errorf("failed to verify %q: %w", pipelinePath, err)
	}

	if response.Valid {
		return true, nil
	}

	if len(response.Errors) == 0 {
		fmt.Fprintf(os.Stderr, "%s: validation failed (no error details returned)\n", pipelinePath)
		return false, fmt.Errorf("validation failed with %d error(s)", 1)
	}

	for _, errMsg := range response.Errors {
		msg := strings.TrimSpace(errMsg)
		if msg == "" {
			fmt.Fprintf(os.Stderr, "%s: validation failed\n", pipelinePath)
			continue
		}

		// Validation service returns locations with "content:<line>:<col>: ...".
		if strings.HasPrefix(msg, "content:") {
			msg = pipelinePath + ":" + strings.TrimPrefix(msg, "content:")
		} else if !strings.Contains(msg, pipelinePath) {
			msg = fmt.Sprintf("%s: %s", pipelinePath, msg)
		}

		fmt.Fprintln(os.Stderr, msg)
	}

	return false, fmt.Errorf("validation failed with %d error(s)", len(response.Errors))
}

func collectYAMLFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if entry.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".yaml" || ext == ".yml" {
			files = append(files, path)
		}

		return nil
	})

	return files, err
}
