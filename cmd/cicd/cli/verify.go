package cli

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/CS7580-SEA-SP26/e-team/internal/common/parser"
	"github.com/CS7580-SEA-SP26/e-team/internal/common/verifier"
	"github.com/spf13/cobra"
)

var verifyCmd = &cobra.Command{
	Use:          "verify [config-file]",
	Short:        "Verify a pipeline configuration file",
	Long:         "Verify that a pipeline configuration file is valid and well-formed",
	Args:         cobra.MaximumNArgs(1),
	RunE:         runVerify,
}

func runVerify(cmd *cobra.Command, args []string) error {
	// get config path
	configPath := ".pipelines/pipeline.yaml"
	if len(args) > 0 {
		configPath = args[0]
	}

	// check Git repo
	if err := checkGitRepo(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return err
	}

	// check if file exists
	absPath, err := filepath.Abs(configPath)
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

	sort.Strings(targets)

	var totalErrors int
	for _, target := range targets {
		p := parser.NewParser(target)
		pipeline, rootNode, parseErr := p.Parse()
		if parseErr != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to parse configuration:\n%s: %v\n", target, parseErr)
			totalErrors++
			continue
		}

		v := verifier.NewPipelineVerifier(target, pipeline, rootNode)
		errors := v.Verify()
		if len(errors) > 0 {
			for _, err := range errors {
				fmt.Fprintln(os.Stderr, err.Error())
			}
			totalErrors += len(errors)
			continue
		}

		if len(targets) == 1 {
			fmt.Println("Configuration is valid ✓")
		} else {
			fmt.Printf("%s: Configuration is valid ✓\n", target)
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

// verifies that we're in a git repository
func checkGitRepo() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	gitDir := filepath.Join(cwd, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return fmt.Errorf("not a git repository (or any parent up to mount point)")
	}

	return nil
}
