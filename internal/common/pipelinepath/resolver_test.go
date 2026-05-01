package pipelinepath

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveInputPath_FileNameMapsToPipelinesDir(t *testing.T) {
	rootDir := t.TempDir()
	pipelineFile := filepath.Join(rootDir, ".pipelines", "build.yaml")
	mustWritePipelineFile(t, pipelineFile)

	resolved, info, err := resolveInputPathFromWorkingDir(rootDir, rootDir, "build.yaml")
	if err != nil {
		t.Fatalf("resolveInputPathFromWorkingDir returned error: %v", err)
	}
	if resolved != pipelineFile {
		t.Fatalf("expected %q, got %q", pipelineFile, resolved)
	}
	if info.IsDir() {
		t.Fatal("expected file target, got directory")
	}
}

func TestResolveInputPath_DotPipelinesDirectoryAllowed(t *testing.T) {
	rootDir := t.TempDir()
	pipelineDir := filepath.Join(rootDir, ".pipelines")
	if err := os.MkdirAll(pipelineDir, 0o755); err != nil {
		t.Fatalf("mkdir pipeline dir: %v", err)
	}

	resolved, info, err := resolveInputPathFromWorkingDir(rootDir, rootDir, ".pipelines")
	if err != nil {
		t.Fatalf("resolveInputPathFromWorkingDir returned error: %v", err)
	}
	if resolved != pipelineDir {
		t.Fatalf("expected %q, got %q", pipelineDir, resolved)
	}
	if !info.IsDir() {
		t.Fatal("expected directory target, got file")
	}
}

func TestResolveInputPath_RejectsNestedDirectoryInput(t *testing.T) {
	rootDir := t.TempDir()
	nestedDir := filepath.Join(rootDir, ".pipelines", "nested")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("mkdir nested dir: %v", err)
	}

	_, _, err := resolveInputPathFromWorkingDir(rootDir, rootDir, ".pipelines/nested")
	if err == nil {
		t.Fatal("expected error for nested directory input, got nil")
	}
	if !strings.Contains(err.Error(), `pipeline directory must be ".pipelines"`) {
		t.Fatalf("expected directory restriction error, got: %v", err)
	}
}

func TestResolveInputPath_RejectsAbsoluteFileOutsidePipelines(t *testing.T) {
	rootDir := t.TempDir()
	outsideFile := filepath.Join(rootDir, "build.yaml")
	if err := os.WriteFile(outsideFile, []byte("pipeline: {}\n"), 0o644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}

	_, _, err := resolveInputPathFromWorkingDir(rootDir, rootDir, outsideFile)
	if err == nil {
		t.Fatal("expected error for absolute file outside .pipelines, got nil")
	}
	if !strings.Contains(err.Error(), `pipeline file must be inside ".pipelines"`) {
		t.Fatalf("expected outside-pipelines error, got: %v", err)
	}
}

func TestResolveInputPath_RejectsParentEscape(t *testing.T) {
	rootDir := t.TempDir()

	_, _, err := resolveInputPathFromWorkingDir(rootDir, rootDir, "../build.yaml")
	if err == nil {
		t.Fatal("expected error for parent escape, got nil")
	}
	if !strings.Contains(err.Error(), "must stay within the repository") {
		t.Fatalf("expected repository boundary error, got: %v", err)
	}
}

func TestResolveInputPath_FromPipelinesWorkingDir_FileNameResolvesInPlace(t *testing.T) {
	rootDir := t.TempDir()
	pipelineDir := filepath.Join(rootDir, ".pipelines")
	pipelineFile := filepath.Join(pipelineDir, "build.yaml")
	mustWritePipelineFile(t, pipelineFile)

	resolved, info, err := resolveInputPathFromWorkingDir(rootDir, pipelineDir, "build.yaml")
	if err != nil {
		t.Fatalf("resolveInputPathFromWorkingDir returned error: %v", err)
	}
	if resolved != pipelineFile {
		t.Fatalf("expected %q, got %q", pipelineFile, resolved)
	}
	if info.IsDir() {
		t.Fatal("expected file target, got directory")
	}
}

func TestResolveInputPath_FromPipelinesWorkingDir_DotResolvesPipelineDir(t *testing.T) {
	rootDir := t.TempDir()
	pipelineDir := filepath.Join(rootDir, ".pipelines")
	if err := os.MkdirAll(pipelineDir, 0o755); err != nil {
		t.Fatalf("mkdir pipeline dir: %v", err)
	}

	resolved, info, err := resolveInputPathFromWorkingDir(rootDir, pipelineDir, ".")
	if err != nil {
		t.Fatalf("resolveInputPathFromWorkingDir returned error: %v", err)
	}
	if resolved != pipelineDir {
		t.Fatalf("expected %q, got %q", pipelineDir, resolved)
	}
	if !info.IsDir() {
		t.Fatal("expected directory target, got file")
	}
}

func mustWritePipelineFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir pipeline parent: %v", err)
	}
	if err := os.WriteFile(path, []byte("pipeline: {}\n"), 0o644); err != nil {
		t.Fatalf("write pipeline file: %v", err)
	}
}
