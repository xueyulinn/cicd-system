package worker

import (
	"context"
	"encoding/binary"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/xueyulinn/cicd-system/internal/models"
	"github.com/moby/moby/client"
)

func newMockDockerClient(t *testing.T, h http.Handler) (*client.Client, func()) {
	t.Helper()
	srv := httptest.NewTLSServer(h)
	cli, err := client.New(
		client.WithHost(srv.URL),
		client.WithAPIVersion("1.53"),
		client.WithHTTPClient(srv.Client()),
	)
	if err != nil {
		srv.Close()
		t.Fatalf("create docker client: %v", err)
	}
	cleanup := func() {
		_ = cli.Close()
		srv.Close()
	}
	return cli, cleanup
}

func dockerFrame(stream byte, payload string) []byte {
	p := []byte(payload)
	buf := make([]byte, 8+len(p))
	buf[0] = stream
	binary.BigEndian.PutUint32(buf[4:], uint32(len(p)))
	copy(buf[8:], p)
	return buf
}

func TestGetLogs_CombinesStdoutAndStderr(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/containers/cid/logs") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(append(dockerFrame(1, "out"), dockerFrame(2, "err")...))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	cli, cleanup := newMockDockerClient(t, h)
	defer cleanup()

	logs, err := getLogs(context.Background(), cli, "cid")
	if err != nil {
		t.Fatalf("getLogs error: %v", err)
	}
	if logs != "out\nerr" {
		t.Fatalf("logs=%q, want %q", logs, "out\\nerr")
	}
}

func TestWaitContainer_NonZeroExit(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/containers/cid/wait") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"StatusCode":2}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	cli, cleanup := newMockDockerClient(t, h)
	defer cleanup()

	err := waitContainer(context.Background(), cli, "cid")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "status 2") {
		t.Fatalf("err=%v", err)
	}
}

func TestExecuteJob_SuccessWithDefaultImage(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/containers/create"):
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"Id":"cid"}`))
		case strings.Contains(r.URL.Path, "/containers/cid/start"):
			w.WriteHeader(http.StatusNoContent)
		case strings.Contains(r.URL.Path, "/containers/cid/wait"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"StatusCode":0}`))
		case strings.Contains(r.URL.Path, "/containers/cid/logs"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(dockerFrame(1, "hello"))
		case strings.Contains(r.URL.Path, "/containers/cid") && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	cli, cleanup := newMockDockerClient(t, h)
	defer cleanup()

	job := &models.JobExecutionPlan{Name: "compile", Script: []string{"echo hello"}}
	logs, err := ExecuteJob(context.Background(), cli, job, "", "", "")
	if err != nil {
		t.Fatalf("ExecuteJob error: %v", err)
	}
	if logs != "hello" {
		t.Fatalf("logs=%q, want %q", logs, "hello")
	}
}

func TestExecuteJob_WaitFailureIncludesLogs(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/containers/create"):
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"Id":"cid"}`))
		case strings.Contains(r.URL.Path, "/containers/cid/start"):
			w.WriteHeader(http.StatusNoContent)
		case strings.Contains(r.URL.Path, "/containers/cid/wait"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"StatusCode":3}`))
		case strings.Contains(r.URL.Path, "/containers/cid/logs"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(dockerFrame(1, "boom"))
		case strings.Contains(r.URL.Path, "/containers/cid") && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	cli, cleanup := newMockDockerClient(t, h)
	defer cleanup()

	job := &models.JobExecutionPlan{Name: "compile", Script: []string{"false"}}
	logs, err := ExecuteJob(context.Background(), cli, job, "", "", "")
	if err == nil {
		t.Fatal("expected error")
	}
	if logs != "boom" {
		t.Fatalf("logs=%q, want boom", logs)
	}
	if !strings.Contains(err.Error(), "wait container") || !strings.Contains(err.Error(), "logs:") {
		t.Fatalf("err=%v", err)
	}
}

func TestExecuteJob_UsesUserProvidedImagePullPath(t *testing.T) {
	pulled := false
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/images/create"):
			pulled = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}\n"))
		case strings.Contains(r.URL.Path, "/containers/create"):
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"Id":"cid"}`))
		case strings.Contains(r.URL.Path, "/containers/cid/start"):
			w.WriteHeader(http.StatusNoContent)
		case strings.Contains(r.URL.Path, "/containers/cid/wait"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"StatusCode":0}`))
		case strings.Contains(r.URL.Path, "/containers/cid/logs"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(dockerFrame(1, "ok"))
		case strings.Contains(r.URL.Path, "/containers/cid") && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	cli, cleanup := newMockDockerClient(t, h)
	defer cleanup()

	job := &models.JobExecutionPlan{Name: "compile", Image: "alpine:3.20", Script: []string{"echo ok"}}
	_, err := ExecuteJob(context.Background(), cli, job, "", "", "")
	if err != nil {
		t.Fatalf("ExecuteJob error: %v", err)
	}
	if !pulled {
		t.Fatal("expected image pull request")
	}
}

func TestExecuteJob_PrepareWorkspaceErrorIncludesContext(t *testing.T) {
	cli, cleanup := newMockDockerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprint(w, "not used")
	}))
	defer cleanup()

	job := &models.JobExecutionPlan{Name: "compile"}
	_, err := ExecuteJob(context.Background(), cli, job, "://bad-url", "abc", "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "prepare workspace") {
		t.Fatalf("err=%v", err)
	}
}
