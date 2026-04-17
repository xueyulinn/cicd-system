package cli

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/CS7580-SEA-SP26/e-team/internal/config"
	"github.com/spf13/cobra"
)

var verifyCmd = &cobra.Command{
	Use:   "verify <pipeline file path>",
	Short: "Verify a pipeline file",
	Long:  "Verify that a pipeline file is ready to run",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runVerify,
}

func runVerify(cmd *cobra.Command, args []string) error {
	// get config path
	configDir := config.DefaultPipelineDir

	// check if file exists
	absPath, err := filepath.Abs(configDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to resolve path: %v\n", err)
		return err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to stat path: %v\n", err)
		return err
	}

	targets := []string{absPath}
	if info.IsDir() {
		targets, err = collectYAMLFiles(absPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to enumerate directory %s: %v\n", absPath, err)
			return err
		}
		if len(targets) == 0 {
			fmt.Fprintf(os.Stderr, "Error: no YAML files found in directory: %s\n", absPath)
			return fmt.Errorf("no YAML files to validate")
		}
	}

	// Create gateway client
	client := NewGatewayClient()

	sort.Strings(targets)

	var totalErrors int
	for _, target := range targets {
		// Read file content for each target
		fileContent, err := os.ReadFile(target)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to read file: %v\n", err)
			return err
		}

		// Call gateway for validation
		response, err := client.Validate(string(fileContent))
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", target, err.Error())
			totalErrors++
			continue
		}

		if !response.Valid {
			for _, errMsg := range response.Errors {
				fmt.Fprintln(os.Stderr, errMsg)
			}
			totalErrors += len(response.Errors)
			continue
		}

		if len(targets) == 1 {
			fmt.Println("Configuration is valid ")
		} else {
			fmt.Printf("%s: Configuration is valid \n", target)
		}
	}

	if totalErrors > 0 {
		return fmt.Errorf("validation failed with %d error(s)", totalErrors)
	}

	if len(targets) > 1 {
		fmt.Println("All configurations are valid ✓")
	}

	return nil
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
