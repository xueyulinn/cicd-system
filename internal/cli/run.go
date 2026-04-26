package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xueyulinn/cicd-system/internal/api"
	"github.com/xueyulinn/cicd-system/internal/common/gitutil"
	"github.com/xueyulinn/cicd-system/internal/common/parser"
	"github.com/xueyulinn/cicd-system/internal/config"
)

var (
	runFile   string
	runName   string
	runBranch string
	runCommit string
	runRemote string
)

var runCmd = &cobra.Command{
	Use:     "run {pipeline-path | pipeline-name} [--branch branch] [--commit commit] [--remote]",
	Short:   "Run a pipeline",
	Long:    "Run a pipeline with the given name or file. For the initial iteration, all pipeline executions happen locally.",
	// Args:    cobra.ExactArgs(1),
	PreRunE: runPreRunE,
	RunE:    runRun,
	DisableFlagsInUseLine: true,
}

// init registers flags for the run command.
func init() {
	runCmd.Flags().StringVarP(&runFile, "file", "f", "", "Pipeline file path")
	runCmd.Flags().StringVarP(&runName, "name", "n", "", "Pipeline name")
	runCmd.Flags().StringVarP(&runBranch, "branch", "b", "", "The Git branch to be used to obtain files for the pipeline run")
	runCmd.Flags().StringVarP(&runCommit, "commit", "c", "", "The Git commit on the branch specified by --branch to be used to obtain files for the pipeline run")
	runCmd.Flags().StringVarP(&runRemote, "remote", "r", "", "The Git remote used to run a remote pipeline")
}

// runPreRunE validates CLI inputs and resolves effective branch/commit values.
func runPreRunE(cmd *cobra.Command, args []string) error {
	repo, err := gitutil.Open(".")

	if err != nil {
		return err
	}
	// Normalize user input to avoid accidental whitespace values.
	runFile = strings.TrimSpace(runFile)
	runName = strings.TrimSpace(runName)
	runBranch = strings.TrimSpace(runBranch)
	runCommit = strings.TrimSpace(runCommit)
	runRemote = strings.TrimSpace(runRemote)

	// must provide --file or --name
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
		resolvedPath, err := findPipelineByName(runName, repo.Root())
		if err != nil {
			return err
		}
		runFile = resolvedPath
	}

	// local run
	if runRemote == "" {
		headBranch, err := repo.GetHeadBranch()
		if err != nil {
			return err
		}

		if runBranch == "" {
			runBranch = headBranch
		}

		headCommit, err := repo.GetHeadCommit()
		if err != nil {
			return err
		}

		if runCommit == "" {
			runCommit = headCommit
		}

		contains, err := repo.BranchContainsCommit(runBranch, runCommit)
		if err != nil {
			return err
		}

		if !contains {
			return fmt.Errorf("commit %q can not be found at branch %q", runCommit, runBranch)
		}
		return nil
	}

	// remote run
	contains, err := repo.RemoteBranchContainsCommit(runRemote, runBranch, runCommit, nil)

	if err != nil {
		return err
	}

	if !contains {
		return fmt.Errorf("commit %q can not be found at branch %q at remote %q", runCommit, runBranch, runRemote)
	}

	return nil
}

// runRun executes a pipeline against a temporary worktree for the resolved
// commit so file reads and job execution use a consistent repository snapshot.
func runRun(cmd *cobra.Command, args []string) error {
	repo, err := gitutil.Open(".")
	if err != nil {
		return err
	}

	client := NewGatewayClient()
	req := api.RunRequest{
		Branch: runBranch,
		Commit: runCommit,
	}

	if runRemote == "" {
		worktree, _, err := repo.CreateDetachedWorktree(runCommit)
		if err != nil {
			return fmt.Errorf("failed to prepare workspace for commit %q: %w", runCommit, err)
		}
		// defer cleanup()

		runFileAtCommit, err := resolveRunFileInWorkspace(runFile, repo.Root(), worktree)
		if err != nil {
			return fmt.Errorf("failed to resolve run file %q in commit workspace: %w", runFile, err)
		}

		fileContent, err := os.ReadFile(runFileAtCommit)
		if err != nil {
			return fmt.Errorf("failed to read run file at commit %q: %w", runCommit, err)
		}

		req.YAMLContent = string(fileContent)
		req.RepoURL = ""
		req.WorkspacePath = worktree
	} else {
		fileContent, err := os.ReadFile(runFile)
		if err != nil {
			return fmt.Errorf("failed to read run file %q: %w", runFile, err)
		}
		remoteURL, err := repo.GetRepoURL(runRemote)
		if err != nil {
			return err
		}

		req.YAMLContent = string(fileContent)
		req.RepoURL = remoteURL
		req.WorkspacePath = ""
	}

	response, err := client.Run(req)
	if err != nil {
		return fmt.Errorf("run failed %w", err)
	}

	if len(response.Errors) > 0 {
		for _, errMsg := range response.Errors {
			fmt.Fprintln(os.Stderr, errMsg)
		}
		return fmt.Errorf("run failed %w", err)
	}

	if response.RunNo != 0 {
		fmt.Printf("Run number: %d\n", response.RunNo)
	}
	if strings.TrimSpace(response.Pipeline) != "" {
		fmt.Printf("Pipeline: %s\n", response.Pipeline)
	}
	if strings.TrimSpace(response.Status) != "" {
		fmt.Printf("Status: %s\n", response.Status)
	}
	if strings.TrimSpace(response.Message) != "" {
		fmt.Println(response.Message)
	}
	return nil
}

// findPipelineByName looks for a pipeline definition by logical name under
// the repository's .pipelines directory and returns the matching file path.
func findPipelineByName(name string, root string) (string, error) {
	pipelineDir := filepath.Join(root, config.DefaultPipelineDir)
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
			cwd, err := os.Getwd()
			if err != nil {
				return filePath, nil
			}
			rel, err := filepath.Rel(cwd, filePath)
			if err != nil {
				return filePath, nil
			}
			return rel, nil
		}
	}

	return "", fmt.Errorf("pipeline with name %s not found", name)
}

// resolveRunFileInWorkspace maps runFile from the current checkout to the same
// repository-relative location inside targetWorkspace.
func resolveRunFileInWorkspace(runFile, repoRoot, targetWorkspace string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	absRunFile := runFile
	if !filepath.IsAbs(absRunFile) {
		absRunFile = filepath.Join(cwd, runFile)
	}
	absRunFile = filepath.Clean(absRunFile)

	rel, err := filepath.Rel(repoRoot, absRunFile)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("run file is outside repository: %s", runFile)
	}

	return filepath.Join(targetWorkspace, rel), nil
}
