package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/CS7580-SEA-SP26/e-team/internal/parser"
	"github.com/CS7580-SEA-SP26/e-team/internal/verifier"
	"github.com/spf13/cobra"
)

var verifyCmd = &cobra.Command{
	Use:   "verify [config-file]",
	Short: "Verify a pipeline configuration file",
	Long:  "Verify that a pipeline configuration file is valid and well-formed",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runVerify,
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

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: configuration file not found: %s\n", configPath)
		return err
	}

	// parse config files
	p := parser.NewParser(configPath)
	pipeline, rootNode, err := p.Parse()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to parse configuration: %v\n", err)
		return err
	}

	// verify the configs
	v := verifier.NewVerifier(configPath, pipeline, rootNode)
	errors := v.Verify()

	if len(errors) > 0 {
		for _, err := range errors {
			fmt.Fprintln(os.Stderr, err.Error())
		}
		return fmt.Errorf("validation failed with %d error(s)", len(errors))
	}

	fmt.Println("Configuration is valid ✓")
	return nil
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
