package worker

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	githttp "github.com/go-git/go-git/v6/plumbing/transport/http"
	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/xueyulinn/cicd-system/internal/models"
)

// DefaultImage is used when the job does not specify an image (no pull, run script only).
const DefaultImage = "alpine:latest"

const (
	envWorkerJobCPULimit          = "WORKER_JOB_CPU_LIMIT"
	envWorkerJobNanoCPUs          = "WORKER_JOB_NANO_CPUS"
	envWorkerJobMemoryLimit       = "WORKER_JOB_MEMORY_LIMIT_MB"
	cpuNanoUnit                   = 1_000_000_000
	memoryMegabyteBytes     int64 = 1024 * 1024
)

// ExecuteJob runs a single job: optionally pull image (if provided), materialize a workspace, run container, wait for exit,
// collect logs, and remove the container.
func (s *Service) ExecuteJob(ctx context.Context, cli *client.Client, job *models.JobExecutionPlan, repoURL, commit, workspacePath string) (logs string, err error) {
	ctx, span := s.tracer.Start(ctx, "execute.job")
	defer span.End()

	if cli == nil || job == nil {
		return "", fmt.Errorf("docker client and job are required")
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
		slog.Error("prepare workspace", "error", err.Error())
		return "", fmt.Errorf("prepare workspace: %w", err)
	}
	if cleanup != nil {
		defer cleanup()
	}

	// 2. Create container
	containerID, err := createContainer(ctx, cli, image, job.Script, workspacePath != "")
	if err != nil {
		return "", fmt.Errorf("create container: %w", err)
	}
	defer func() {
		_ = removeContainer(context.Background(), cli, containerID)
	}()

	if workspacePath != "" {
		if err := copyWorkspaceToContainer(ctx, cli, containerID, workspacePath); err != nil {
			slog.Error("copy workspace to container", "error", err.Error())
			return "", fmt.Errorf("copy workspace to container: %w", err)
		}
	}

	if err := startContainer(ctx, cli, containerID); err != nil {
		return "", fmt.Errorf("run container: %w", err)
	}

	// 3. Wait for container to exit, then always attempt to collect logs so
	// failed jobs expose stderr/stdout to callbacks and debugging output.
	waitErr := waitContainer(ctx, cli, containerID)

	// 4. Get logs
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
			URL:  repoURL,
			Auth: cloneAuth(repoURL),
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

	resolvedPath, err := resolveWorkspacePath(workspacePath)
	if err != nil {
		return "", nil, err
	}
	return resolvedPath, nil, nil
}

func resolveWorkspacePath(workspacePath string) (string, error) {
	workspacePath = strings.TrimSpace(workspacePath)
	if workspacePath == "" {
		return "", nil
	}

	if _, err := os.Stat(workspacePath); err == nil {
		return workspacePath, nil
	}

	if !looksLikeWindowsPath(workspacePath) {
		return workspacePath, nil
	}

	mappedPath := mapWindowsWorkspacePath(workspacePath)
	if mappedPath == "" {
		return workspacePath, nil
	}
	if _, err := os.Stat(mappedPath); err == nil {
		return mappedPath, nil
	}

	return workspacePath, nil
}

func mapWindowsWorkspacePath(workspacePath string) string {
	hostTempDir := strings.TrimSpace(os.Getenv("WORKSPACE_HOST_TEMP_DIR"))
	if hostTempDir == "" {
		hostTempDir = "/host-temp"
	}

	normalized := strings.ReplaceAll(strings.TrimSpace(workspacePath), "\\", "/")
	base := path.Base(normalized)
	if base == "." || base == "/" || base == "" {
		return ""
	}

	return filepath.Join(hostTempDir, base)
}

func looksLikeWindowsPath(p string) bool {
	if strings.Contains(p, "\\") {
		return true
	}
	if len(p) < 2 || p[1] != ':' {
		return false
	}
	return (p[0] >= 'a' && p[0] <= 'z') || (p[0] >= 'A' && p[0] <= 'Z')
}

func cloneAuth(repoURL string) *githttp.BasicAuth {
	if username := strings.TrimSpace(os.Getenv("GIT_USERNAME")); username != "" {
		return &githttp.BasicAuth{
			Username: username,
			Password: strings.TrimSpace(os.Getenv("GIT_PASSWORD")),
		}
	}

	if strings.Contains(strings.ToLower(strings.TrimSpace(repoURL)), "github.com") {
		if token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); token != "" {
			return &githttp.BasicAuth{
				Username: "x-access-token",
				Password: token,
			}
		}
	}

	return nil
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

// createContainer creates a container for the given image and script; returns container ID.
// If hasWorkspace is true, the container will run with /workspace as its working directory.
func createContainer(ctx context.Context, cli *client.Client, image string, script []string, hasWorkspace bool) (containerID string, err error) {
	resources, err := containerResourcesFromEnv()
	if err != nil {
		return "", err
	}

	cmd := scriptToCmd(script)
	cfg := &container.Config{
		Image:        image,
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	}
	if hasWorkspace {
		cfg.WorkingDir = "/workspace"
	}

	opts := client.ContainerCreateOptions{Config: cfg}
	if hasContainerResourceLimits(resources) {
		opts.HostConfig = &container.HostConfig{
			Resources: resources,
		}
	}

	createResp, err := cli.ContainerCreate(ctx, opts)
	if err != nil {
		slog.Error("created container failed: ", "error", err.Error())
		return "", err
	}
	return createResp.ID, nil
}

func containerResourcesFromEnv() (container.Resources, error) {
	var resources container.Resources

	nanoCPURaw := strings.TrimSpace(os.Getenv(envWorkerJobNanoCPUs))
	cpuLimitRaw := strings.TrimSpace(os.Getenv(envWorkerJobCPULimit))
	memoryLimitRaw := strings.TrimSpace(os.Getenv(envWorkerJobMemoryLimit))

	if nanoCPURaw != "" && cpuLimitRaw != "" {
		return resources, fmt.Errorf("%s and %s cannot both be set", envWorkerJobNanoCPUs, envWorkerJobCPULimit)
	}

	if nanoCPURaw != "" {
		nanoCPUs, err := strconv.ParseInt(nanoCPURaw, 10, 64)
		if err != nil || nanoCPUs <= 0 {
			return resources, fmt.Errorf("%s must be a positive int64, got %q", envWorkerJobNanoCPUs, nanoCPURaw)
		}
		resources.NanoCPUs = nanoCPUs
	}

	if cpuLimitRaw != "" {
		cpuLimit, err := strconv.ParseFloat(cpuLimitRaw, 64)
		if err != nil || cpuLimit <= 0 {
			return resources, fmt.Errorf("%s must be a positive number, got %q", envWorkerJobCPULimit, cpuLimitRaw)
		}
		resources.NanoCPUs = int64(math.Round(cpuLimit * cpuNanoUnit))
		if resources.NanoCPUs <= 0 {
			return resources, fmt.Errorf("%s resolved to invalid NanoCPUs for value %q", envWorkerJobCPULimit, cpuLimitRaw)
		}
	}

	if memoryLimitRaw != "" {
		memoryLimitMB, err := strconv.ParseInt(memoryLimitRaw, 10, 64)
		if err != nil || memoryLimitMB <= 0 {
			return resources, fmt.Errorf("%s must be a positive integer (MB), got %q", envWorkerJobMemoryLimit, memoryLimitRaw)
		}
		resources.Memory = memoryLimitMB * memoryMegabyteBytes
	}

	return resources, nil
}

func hasContainerResourceLimits(resources container.Resources) bool {
	return resources.NanoCPUs > 0 || resources.Memory > 0
}

func startContainer(ctx context.Context, cli *client.Client, containerID string) error {
	_, err := cli.ContainerStart(ctx, containerID, client.ContainerStartOptions{})
	return err
}

func copyWorkspaceToContainer(ctx context.Context, cli *client.Client, containerID string, workspacePath string) error {
	archiveBuf, err := buildWorkspaceArchive(workspacePath)
	if err != nil {
		return err
	}

	_, err = cli.CopyToContainer(ctx, containerID, client.CopyToContainerOptions{
		DestinationPath: "/",
		Content:         archiveBuf,
	})
	return err
}

func buildWorkspaceArchive(workspacePath string) (io.Reader, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	err := filepath.Walk(workspacePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(workspacePath, path)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)
		if relPath == "." {
			return nil
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(filepath.Join("workspace", relPath))

		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() { _ = file.Close() }()

		_, err = io.Copy(tw, file)
		return err
	})
	if err != nil {
		_ = tw.Close()
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}

	return bytes.NewReader(buf.Bytes()), nil
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
