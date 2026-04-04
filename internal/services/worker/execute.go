package worker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/CS7580-SEA-SP26/e-team/internal/models"
	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/client"
)

// DefaultImage is used when the job does not specify an image (no pull, run script only).
const DefaultImage = "alpine:latest"

// ExecuteJob runs a single job: optionally pull image (if provided), materialize a workspace, run container, wait for exit,
// collect logs, and remove the container.
func ExecuteJob(ctx context.Context, cli *client.Client, job *models.JobExecutionPlan, repoURL, commit, workspacePath string) (logs string, err error) {
	if cli == nil || job == nil {
		return "", fmt.Errorf("client and job are required")
	}

	image := job.Image
	if image == "" {
		image = DefaultImage
	}

	// 1. Pull image only when user provided one
	if job.Image != "" {
		if err := pullImage(ctx, cli, image); err != nil {
			return "", fmt.Errorf("pull image %q: %w", image, err)
		}
	}

	workspacePath, cleanup, err := materializeWorkspace(ctx, repoURL, commit, workspacePath)
	if err != nil {
		return "", fmt.Errorf("prepare workspace: %w", err)
	}
	if cleanup != nil {
		defer cleanup()
	}

	// 2. Create and start container
	containerID, err := runContainer(ctx, cli, image, job.Script, workspacePath)
	if err != nil {
		return "", fmt.Errorf("run container: %w", err)
	}
	defer func() {
		_ = removeContainer(context.Background(), cli, containerID)
	}()

	// 3. Wait for container to exit, then always attempt to collect logs so
	// failed jobs expose stderr/stdout to callbacks and debugging output.
	waitErr := waitContainer(ctx, cli, containerID)

	// 4. Get logs (stdout/stderr)
	logs, logsErr := getLogs(ctx, cli, containerID)
	if logsErr != nil {
		if waitErr != nil {
			return "", fmt.Errorf("wait container: %w; get logs: %v", waitErr, logsErr)
		}
		return "", fmt.Errorf("get logs: %w", logsErr)
	}
	if waitErr != nil {
		if strings.TrimSpace(logs) != "" {
			return logs, fmt.Errorf("wait container: %w\nlogs:\n%s", waitErr, logs)
		}
		return logs, fmt.Errorf("wait container: %w", waitErr)
	}

	return logs, nil
}

func materializeWorkspace(ctx context.Context, repoURL, commit, workspacePath string) (string, func(), error) {
	if strings.TrimSpace(repoURL) != "" && strings.TrimSpace(commit) != "" {
		tmpDir, err := os.MkdirTemp("", "cicd-worker-repo-*")
		if err != nil {
			return "", nil, err
		}

		repo, err := git.PlainCloneContext(ctx, tmpDir, &git.CloneOptions{
			URL: repoURL,
		})
		if err != nil {
			_ = os.RemoveAll(tmpDir)
			return "", nil, err
		}

		wt, err := repo.Worktree()
		if err != nil {
			_ = os.RemoveAll(tmpDir)
			return "", nil, err
		}

		if err := wt.Checkout(&git.CheckoutOptions{
			Hash:  plumbing.NewHash(commit),
			Force: true,
		}); err != nil {
			_ = os.RemoveAll(tmpDir)
			return "", nil, err
		}

		return tmpDir, func() { _ = os.RemoveAll(tmpDir) }, nil
	}

	return workspacePath, nil, nil
}

// scriptToCmd converts script lines (e.g. ["make build", "make test"]) into Docker Cmd.
// Docker's Cmd expects the first element to be the executable; we run via sh -c so that
// "make build" is executed as a shell command, not as an executable named "make build".
func scriptToCmd(script []string) []string {
	if len(script) == 0 {
		return []string{"sh", "-c", "true"}
	}
	one := strings.TrimSpace(strings.Join(script, " && "))
	if one == "" {
		return []string{"sh", "-c", "true"}
	}
	return []string{"sh", "-c", one}
}

// pullImage pulls the image so it is available for the container.
func pullImage(ctx context.Context, cli *client.Client, imageRef string) error {
	resp, err := cli.ImagePull(ctx, imageRef, client.ImagePullOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = resp.Close() }()
	return resp.Wait(ctx)
}

// runContainer creates and starts a container with the given image and script; returns container ID.
// If workspacePath is non-empty, it is bound to /workspace and set as WorkingDir.
func runContainer(ctx context.Context, cli *client.Client, image string, script []string, workspacePath string) (containerID string, err error) {
	cmd := scriptToCmd(script)
	cfg := &container.Config{
		Image:        image,
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	}
	if workspacePath != "" {
		cfg.WorkingDir = "/workspace"
	}
	opts := client.ContainerCreateOptions{Config: cfg}
	if workspacePath != "" {
		opts.HostConfig = &container.HostConfig{
			Mounts: []mount.Mount{
				{
					Type:   mount.TypeBind,
					Source: workspacePath,
					Target: "/workspace",
				},
			},
		}
	}
	createResp, err := cli.ContainerCreate(ctx, opts)
	if err != nil {
		return "", err
	}
	containerID = createResp.ID
	_, err = cli.ContainerStart(ctx, containerID, client.ContainerStartOptions{})
	if err != nil {
		return containerID, err
	}
	return containerID, nil
}

// waitContainer blocks until the container exits.
func waitContainer(ctx context.Context, cli *client.Client, containerID string) error {
	wait := cli.ContainerWait(ctx, containerID, client.ContainerWaitOptions{
		Condition: container.WaitConditionNotRunning,
	})
	select {
	case err := <-wait.Error:
		return err
	case result := <-wait.Result:
		if result.StatusCode != 0 {
			return fmt.Errorf("container exited with status %s", strconv.FormatInt(result.StatusCode, 10))
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// getLogs returns the container's stdout and stderr.
func getLogs(ctx context.Context, cli *client.Client, containerID string) (string, error) {
	rc, err := cli.ContainerLogs(ctx, containerID, client.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		return "", err
	}
	defer func() { _ = rc.Close() }()
	var out, errOut bytes.Buffer
	_, err = stdcopy.StdCopy(&out, &errOut, rc)
	if err != nil && err != io.EOF {
		return "", err
	}
	// Combine stdout and stderr; stderr after stdout.
	logs := out.String()
	if errOut.Len() > 0 {
		if logs != "" {
			logs += "\n"
		}
		logs += errOut.String()
	}
	return logs, nil
}

// removeContainer removes the container.
func removeContainer(ctx context.Context, cli *client.Client, containerID string) error {
	_, err := cli.ContainerRemove(ctx, containerID, client.ContainerRemoveOptions{Force: true})
	return err
}
