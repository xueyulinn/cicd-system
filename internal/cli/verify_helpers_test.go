package cli

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestCollectYAMLFiles(t *testing.T) {
	dir := t.TempDir()
	mustWrite := func(path string) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	mustWrite(filepath.Join(dir, "a.yaml"))
	mustWrite(filepath.Join(dir, "b.yml"))
	mustWrite(filepath.Join(dir, "c.txt"))
	mustWrite(filepath.Join(dir, "nested", "d.yaml"))

	files, err := collectYAMLFiles(dir)
	if err != nil {
		t.Fatalf("collectYAMLFiles err=%v", err)
	}
	sort.Strings(files)
	if len(files) != 3 {
		t.Fatalf("files=%v", files)
	}
	joined := strings.Join(files, "\n")
	if !strings.Contains(joined, "a.yaml") || !strings.Contains(joined, "b.yml") || !strings.Contains(joined, "d.yaml") {
		t.Fatalf("unexpected files: %v", files)
	}
}

func TestCheckGitRepo(t *testing.T) {
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()

	t.Run("not git repo", func(t *testing.T) {
		d := t.TempDir()
		wd, _ := os.Getwd()
		defer func() { _ = os.Chdir(wd) }()
		if err := os.Chdir(d); err != nil {
			t.Fatalf("chdir: %v", err)
		}
		err := checkGitRepo()
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("git repo", func(t *testing.T) {
		d := t.TempDir()
		wd, _ := os.Getwd()
		defer func() { _ = os.Chdir(wd) }()
		if err := os.Mkdir(filepath.Join(d, ".git"), 0o755); err != nil {
			t.Fatalf("mkdir .git: %v", err)
		}
		if err := os.Chdir(d); err != nil {
			t.Fatalf("chdir: %v", err)
		}
		if err := checkGitRepo(); err != nil {
			t.Fatalf("unexpected err=%v", err)
		}
	})
}
