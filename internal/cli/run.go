package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xueyulinn/cicd-system/internal/api"
	"github.com/xueyulinn/cicd-system/internal/common/gitutil"
	"github.com/xueyulinn/cicd-system/internal/common/parser"
	"github.com/xueyulinn/cicd-system/internal/common/pipelinepath"
	"github.com/xueyulinn/cicd-system/internal/common/snapshot"
	"github.com/xueyulinn/cicd-system/internal/config"
	"github.com/xueyulinn/cicd-system/internal/objectstorage"
)

var (
	runFile   string
	runName   string
	runBranch string
	runCommit string
	runRemote string
)

var runCmd = &cobra.Command{
	Use:   "run {--file pipeline-path | --name pipeline-name} [--branch branch | --commit commit] [--remote]",
	Short: "Run a pipeline",
	Long:  "Run a pipeline with the given name or file. For the initial iteration, all pipeline executions happen locally.",
	// Args:    cobra.ExactArgs(1),
	PreRunE:               runPreRunE,
	RunE:                  runRun,
	DisableFlagsInUseLine: true,
}

// init registers flags for the run command.
func init() {
	runCmd.Flags().StringVarP(&runFile, "file", "f", "", "Pipeline file path")
	runCmd.Flags().StringVarP(&runName, "name", "n", "", "Pipeline name")
	runCmd.Flags().StringVarP(&runBranch, "branch", "b", "", "Resolve the run commit from the tip of the specified Git branch")
	runCmd.Flags().StringVarP(&runCommit, "commit", "c", "", "Run the specified Git commit directly")
	runCmd.Flags().StringVarP(&runRemote, "remote", "r", "", "The Git remote used to run a remote pipeline")
}

// runPreRunE validates CLI inputs and resolves the effective commit value.
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
	if runBranch != "" && runCommit != "" {
		return fmt.Errorf("must provide exactly one --branch or --commit")
	}

	// --name must be a valid pipeline name
	if runName != "" {
		resolvedPath, err := findPipelineByName(runName, repo.Root())
		if err != nil {
			return err
		}
		runFile = resolvedPath
	}

	if runCommit == "" {
		switch {
		case runBranch != "":
			if runRemote == "" {
				runCommit, err = repo.GetHeadCommitByBranch(runBranch)
			} else {
				runCommit, err = repo.GetRemoteHeadCommitByBranch(runRemote, runBranch, nil)
			}
			if err != nil {
				return fmt.Errorf("failed to resolve commit for branch %q: %w", runBranch, err)
			}
		default:
			runCommit, err = repo.GetHeadCommit()
			if err != nil {
				return fmt.Errorf("failed to get head commit: %w", err)
			}
		}
	}

	return nil
}

// runRun executes a pipeline against a temporary worktree for the resolved
// commit so file reads and job execution use a consistent repository snapshot.
func runRun(cmd *cobra.Command, args []string) error {
	repo, ok := cmd.Context().Value(repoKey).(*gitutil.Repository)
	if !ok || repo == nil {
		return fmt.Errorf("git repository context is missing")
	}
	rootDir := repo.Root()

	client := NewGatewayClient()
	req := api.RunRequest{
		Commit: runCommit,
	}

	if runRemote == "" {
		worktree, cleanup, err := repo.CreateDetachedWorktree(runCommit)
		if err != nil {
			return fmt.Errorf("failed to prepare workspace for commit %q: %w", runCommit, err)
		}
		defer cleanup()

		completePath, _, err := pipelinepath.ResolveInputPath(rootDir, runFile)
		if err != nil {
			return err
		}

		pipelineData, err := repo.ReadFileAtCommit(runCommit, completePath)
		if err != nil {
			return fmt.Errorf("read pipeline file failed: %w", err)
		}

		objectName, err := uploadWorkspace(cmd.Context(), worktree, runCommit) 
		if err != nil {
			return fmt.Errorf("upload workspace snapshot: %w", err)
		}

		req.YAMLContent = string(pipelineData)
		req.RepoURL = ""
		req.WorkspaceObjectName = objectName
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
		req.WorkspaceObjectName = ""
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

func uploadWorkspace(ctx context.Context, worktree, commit string) (string, error) {
	minioClient, err := objectstorage.NewMinioClient(objectstorage.LoadConfig())
	objectName := minioClient.BuildObjectName(commit)
	if err != nil {
		return objectName, err
	}

	stagingDir, err := os.MkdirTemp("", "cicd-run-upload-*")
	if err != nil {
		return objectName, err
	}
	defer os.RemoveAll(stagingDir)

	archivePath := filepath.Join(stagingDir, "workspace.tar.gz")
	if err := snapshot.Pack(worktree, archivePath); err != nil {
		return objectName, err
	}

	if err := minioClient.UploadWorkspace(ctx, objectName, archivePath); err != nil {
		return objectName, err
	}

	return objectName, nil
}
