package worker

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/CS7580-SEA-SP26/e-team/internal/models"
	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

// ExecuteJob runs a single job: pull image, run container, wait for exit, collect logs, remove container.
// Returns the container stdout/stderr and any error. Container is always removed on return (best effort).
func ExecuteJob(ctx context.Context, cli *client.Client, job *models.JobExecutionPlan) (logs string, err error) {
	if cli == nil || job == nil {
		return "", fmt.Errorf("client and job are required")
	}

	// 1. Pull image
	if err := pullImage(ctx, cli, job.Image); err != nil {
		return "", fmt.Errorf("pull image %q: %w", job.Image, err)
	}

	// 2. Create and start container
	containerID, err := runContainer(ctx, cli, job.Image, job.Script)
	if err != nil {
		return "", fmt.Errorf("run container: %w", err)
	}
	defer func() {
		_ = removeContainer(context.Background(), cli, containerID)
	}()

	// 3. Wait for container to exit
	if err := waitContainer(ctx, cli, containerID); err != nil {
		return "", fmt.Errorf("wait container: %w", err)
	}

	// 4. Get logs (stdout/stderr)
	logs, err = getLogs(ctx, cli, containerID)
	if err != nil {
		return "", fmt.Errorf("get logs: %w", err)
	}

	return logs, nil
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
func runContainer(ctx context.Context, cli *client.Client, image string, script []string) (containerID string, err error) {
	cfg := &container.Config{
		Image:        image,
		Cmd:          script,
		AttachStdout: true,
		AttachStderr: true,
	}
	createResp, err := cli.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config: cfg,
		Image:  image,
	})
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
	case <-wait.Result:
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
