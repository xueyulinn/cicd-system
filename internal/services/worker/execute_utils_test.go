package worker

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestScriptToCmd(t *testing.T) {
	cases := []struct {
		name   string
		script []string
		want   []string
	}{
		{"empty", nil, []string{"sh", "-c", "true"}},
		{"blank", []string{"  "}, []string{"sh", "-c", "true"}},
		{"single", []string{"go test ./..."}, []string{"sh", "-c", "go test ./..."}},
		{"multi", []string{"go test ./...", "go vet ./..."}, []string{"sh", "-c", "go test ./... && go vet ./..."}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := scriptToCmd(tc.script); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got=%v want=%v", got, tc.want)
			}
		})
	}
}

func TestMaterializeWorkspaceWithoutRepoClone(t *testing.T) {
	path, cleanup, err := materializeWorkspace(context.Background(), "", "", "/tmp/ws")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if path != "/tmp/ws" || cleanup != nil {
		t.Fatalf("path=%q cleanup-nil=%v", path, cleanup == nil)
	}
}

func TestBuildWorkspaceArchiveContainsWorkspacePrefixAndFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "nested"), 0o755); err != nil {
		t.Fatalf("mkdir err=%v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("alpha"), 0o644); err != nil {
		t.Fatalf("write a err=%v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "nested", "b.txt"), []byte("beta"), 0o644); err != nil {
		t.Fatalf("write b err=%v", err)
	}

	r, err := buildWorkspaceArchive(root)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	blob, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read err=%v", err)
	}

	tr := tar.NewReader(bytes.NewReader(blob))
	entries := map[string]string{}
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar err=%v", err)
		}
		if h.FileInfo().IsDir() {
			continue
		}
		content, err := io.ReadAll(tr)
		if err != nil {
			t.Fatalf("entry read err=%v", err)
		}
		entries[h.Name] = string(content)
	}

	if entries["workspace/a.txt"] != "alpha" {
		t.Fatalf("a=%q", entries["workspace/a.txt"])
	}
	if entries["workspace/nested/b.txt"] != "beta" {
		t.Fatalf("b=%q", entries["workspace/nested/b.txt"])
	}
}
