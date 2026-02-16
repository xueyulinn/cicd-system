package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/CS7580-SEA-SP26/e-team/internal/common/parser"
	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/spf13/cobra"
)

var (
	runFile string
	runName string
	runBranch string
	runCommit string
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Execute a pipeline locally",
	Long:  "Run a pipeline with the given name or file. For the initial iteration, all pipeline executions happen locally.",
	Args:  cobra.NoArgs,
	PreRunE: runPreRunE,
	RunE:  runRun,
}

func runRun(cmd *cobra.Command, args []string) error {
	fileContent, err := os.ReadFile(runFile)
	if err != nil {
		return fmt.Errorf("failed to read pipeline file: %w", err)
	}

	workspacePath, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get workspace: %w", err)
	}
	reqBody := runRequest{
		YAMLContent:   string(fileContent),
		Branch:        runBranch,
		Commit:        runCommit,
		WorkspacePath: workspacePath,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal run request: %w", err)
	}

	executionURL := strings.TrimSpace(os.Getenv("EXECUTION_URL"))
	if executionURL == "" {
		executionURL = "http://localhost:8002"
	}
	executionURL = strings.TrimRight(executionURL, "/")

	// Pipeline can take several minutes (pull image, build, multiple test jobs).
	client := &http.Client{Timeout: 15 * time.Minute}
	resp, err := client.Post(executionURL+"/run", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to call execution service: %w", err)
	}
	defer func() {
		_ = resp.Body.Close() // Ignore close error as we're done with the body
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read execution response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if len(respBody) > 0 {
			return fmt.Errorf("execution service returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
		}
		return fmt.Errorf("execution service returned status %d", resp.StatusCode)
	}

	var runResp runResponse
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &runResp); err == nil {
			if !runResp.Success {
				if len(runResp.Errors) > 0 {
					return fmt.Errorf("run failed: %s", strings.Join(runResp.Errors, "; "))
				}
				return fmt.Errorf("run failed")
			}
			if strings.TrimSpace(runResp.Message) != "" {
				fmt.Println(runResp.Message)
			} else {
				fmt.Println("Run completed successfully.")
			}
			return nil
		}

		// Fallback for non-JSON success responses.
		fmt.Println(string(respBody))
		return nil
	}

	fmt.Println("Run request submitted successfully.")
	return nil
}

type runRequest struct {
	YAMLContent   string `json:"yaml_content"`
	Branch        string `json:"branch"`
	Commit        string `json:"commit"`
	WorkspacePath string `json:"workspace_path,omitempty"`
}

type runResponse struct {
	Success bool     `json:"success"`
	Errors  []string `json:"errors,omitempty"`
	Message string   `json:"message,omitempty"`
}

func init() {
	runCmd.Flags().StringVar(&runFile, "file", "", "Pipeline file path")
	runCmd.Flags().StringVar(&runName, "name", "", "Pipeline name")
	runCmd.Flags().StringVar(&runBranch, "branch", "", "The Git branch to be used to obtain files for the pipeline run")
	runCmd.Flags().StringVar(&runCommit, "commit", "", "The Git commit on the branch specified by --branch to be used to obtain files for the pipeline run")
}

func runPreRunE(cmd *cobra.Command, args []string) error {
	// Normalize user input to avoid accidental whitespace values.
	runFile = strings.TrimSpace(runFile)
	runName = strings.TrimSpace(runName)
	runBranch = strings.TrimSpace(runBranch)
	runCommit = strings.TrimSpace(runCommit)

	// cannot provide both --file and --name
	if runFile == "" && runName == "" {
		return fmt.Errorf("must provide at least one of --file or --name")
	}

	// cannot provide both --file and --name
	if runFile != "" && runName != "" {
		return fmt.Errorf("must provide exactly one --file or --name")
	}

	// --file must be a valid file path
	if runFile != "" {
		if _, err := os.Stat(runFile); err != nil {
			return fmt.Errorf("invalid --file: %s", runFile)
		}
	}

	// --name must be a valid pipeline name
	if runName != "" {
		resolvedPath, err := findPipelineByName(runName)
		if err != nil {
			return err
		}
		runFile = resolvedPath
	}

	// get current branch
	currentBranch, err := getCurrentBranch()
	if err != nil {
		return err
	}

	if runBranch != "" && runBranch != currentBranch {
		return fmt.Errorf(
			"--branch %q does not match current checked out branch %q",
			runBranch,
			currentBranch,
		)
	}

	// if branch is not provided, use main branch
	if runBranch == "" {
		runBranch = "main"
	}

	// get current commit
	currentCommit, err := getCurrentCommit()
	if err != nil {
		return err
	}

	// if commit is provided, check if it matches the current commit
	if runCommit != "" && runCommit != currentCommit {
		return fmt.Errorf(
			"--commit %q does not match current checked out commit %q",
			runCommit,
			currentCommit,
		)
	}

	// if commit is not provided, get the latest commit on the branch
	if runCommit == "" {
		latestCommit, err := getLatestCommitByBranch(runBranch)
		if err != nil {
			return err
		}
		runCommit = latestCommit
	}

	return nil
}

// findPipelineByName looks for a pipeline definition by logical name under
// the repository's .pipelines directory and returns the matching file path.
func findPipelineByName(name string) (string, error) {
	pipelineDir := ".pipelines"
	files, err := os.ReadDir(pipelineDir)
	if err != nil {
		return "", fmt.Errorf("failed to read pipeline directory: %w", err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filePath := filepath.Join(pipelineDir, file.Name())
		p := parser.NewParser(filePath)
		pipeline, _, err := p.Parse()
		if err != nil {
			continue	
		}
		
		if pipeline.Name == name {
			return filePath, nil
		}
	}

	return "", fmt.Errorf("pipeline with name %s not found", name)
}

// getCurrentBranch returns the name of the current branch from the HEAD reference.
func getCurrentBranch() (string, error) {
	repo, err := git.PlainOpen(".")
	if err != nil {
		return "", fmt.Errorf("failed to open git repo: %w", err)
	}

	ref, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD: %w", err)
	}

	if ref.Name().IsBranch() {
		return ref.Name().Short(), nil
	}

	return "", fmt.Errorf("detached HEAD at %s", ref.Hash().String())
}

// getCurrentCommit returns the commit hash currently checked out at HEAD.
func getCurrentCommit() (string, error) {
	repo, err := git.PlainOpen(".")
	if err != nil {
		return "", fmt.Errorf("failed to open git repo: %w", err)
	}

	ref, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD: %w", err)
	}

	return ref.Hash().String(), nil
}

// getLatestCommitByBranch returns the tip commit hash of a local branch from
// refs/heads/<branch> in the current repository.
func getLatestCommitByBranch(branch string) (string, error) {
	repo, err := git.PlainOpen(".")
	if err != nil {
		return "", fmt.Errorf("failed to open git repo: %w", err)
	}

	// refs/heads/branch
	localRefName := plumbing.NewBranchReferenceName(branch)
	ref, err := repo.Reference(localRefName, true)
	if err == nil {
		return ref.Hash().String(), nil
	}

	return "", fmt.Errorf("local branch %q not found: %w", branch, err)
}
