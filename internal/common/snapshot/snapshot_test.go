package snapshot

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPack(t *testing.T) {
	sourceDir := createTestWorkspace(t)
	archivePath := filepath.Join(t.TempDir(), "workspace.tar.gz")

	if err := Pack(sourceDir, archivePath); err != nil {
		t.Fatalf("Pack() error = %v", err)
	}

	entries := archiveEntries(t, archivePath)

	assertHasEntry(t, entries, ".", tar.TypeDir)
	assertHasEntry(t, entries, "root.txt", tar.TypeReg)
	assertHasEntry(t, entries, "nested", tar.TypeDir)
	assertHasEntry(t, entries, "nested/child.txt", tar.TypeReg)
}

func TestUnpack(t *testing.T) {
	sourceDir := createTestWorkspace(t)
	archivePath := filepath.Join(t.TempDir(), "workspace.tar.gz")

	if err := Pack(sourceDir, archivePath); err != nil {
		t.Fatalf("Pack() error = %v", err)
	}

	destinationDir := filepath.Join(t.TempDir(), "unzipped")
	if err := Unpack(archivePath, destinationDir); err != nil {
		t.Fatalf("Unpack() error = %v", err)
	}

	assertFileContent(t, filepath.Join(destinationDir, "root.txt"), "root file")
	assertFileContent(t, filepath.Join(destinationDir, "nested", "child.txt"), "child file")
}

func TestPack_SkipsOutputArchiveInsideSource(t *testing.T) {
	sourceDir := createTestWorkspace(t)
	archivePath := filepath.Join(sourceDir, "workspace.tar.gz")

	if err := Pack(sourceDir, archivePath); err != nil {
		t.Fatalf("Pack() error = %v", err)
	}

	entries := archiveEntries(t, archivePath)
	if _, ok := entries["workspace.tar.gz"]; ok {
		t.Fatalf("archive unexpectedly contains itself")
	}
}

func TestUnpack_RejectsPathTraversal(t *testing.T) {
	archivePath := createArchive(t, []archiveEntry{
		{
			name:     "../escape.txt",
			typeflag: tar.TypeReg,
			body:     "escape",
			mode:     0o644,
		},
	})

	destinationDir := filepath.Join(t.TempDir(), "unzipped")
	err := Unpack(archivePath, destinationDir)
	if err == nil {
		t.Fatalf("Unpack() error = nil, want path traversal rejection")
	}
	if !strings.Contains(err.Error(), "outside destination") {
		t.Fatalf("Unpack() error = %v, want outside destination rejection", err)
	}
}

func TestUnpack_RejectsUnsupportedEntries(t *testing.T) {
	archivePath := createArchive(t, []archiveEntry{
		{
			name:     "link",
			typeflag: tar.TypeSymlink,
			mode:     0o777,
		},
	})

	destinationDir := filepath.Join(t.TempDir(), "unzipped")
	err := Unpack(archivePath, destinationDir)
	if err == nil {
		t.Fatalf("Unpack() error = nil, want unsupported entry rejection")
	}
	if !strings.Contains(err.Error(), "unsupported tar entry type") {
		t.Fatalf("Unpack() error = %v, want unsupported entry rejection", err)
	}
}

func createTestWorkspace(t *testing.T) string {
	t.Helper()

	root := t.TempDir()

	nestedDir := filepath.Join(root, "nested")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, "root.txt"), []byte("root file"), 0o644); err != nil {
		t.Fatalf("WriteFile(root.txt) error = %v", err)
	}

	if err := os.WriteFile(filepath.Join(nestedDir, "child.txt"), []byte("child file"), 0o644); err != nil {
		t.Fatalf("WriteFile(child.txt) error = %v", err)
	}

	return root
}

func archiveEntries(t *testing.T, archivePath string) map[string]byte {
	t.Helper()

	f, err := os.Open(archivePath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			t.Fatalf("file.Close() error = %v", closeErr)
		}
	}()

	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("gzip.NewReader() error = %v", err)
	}
	defer func() {
		if closeErr := gz.Close(); closeErr != nil {
			t.Fatalf("gzip.Close() error = %v", closeErr)
		}
	}()

	tr := tar.NewReader(gz)

	entries := make(map[string]byte)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return entries
		}
		if err != nil {
			t.Fatalf("tar.Next() error = %v", err)
		}

		entries[header.Name] = header.Typeflag
	}
}

type archiveEntry struct {
	name     string
	typeflag byte
	body     string
	mode     int64
}

func createArchive(t *testing.T, entries []archiveEntry) string {
	t.Helper()

	archivePath := filepath.Join(t.TempDir(), "archive.tar.gz")

	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)

	for _, entry := range entries {
		header := &tar.Header{
			Name:     entry.name,
			Typeflag: entry.typeflag,
			Mode:     entry.mode,
			Size:     int64(len(entry.body)),
		}
		if err := tw.WriteHeader(header); err != nil {
			t.Fatalf("WriteHeader(%q) error = %v", entry.name, err)
		}
		if entry.body == "" {
			continue
		}
		if _, err := io.Copy(tw, bytes.NewBufferString(entry.body)); err != nil {
			t.Fatalf("Copy(%q) error = %v", entry.name, err)
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("tar.Close() error = %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip.Close() error = %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("file.Close() error = %v", err)
	}

	return archivePath
}

func assertHasEntry(t *testing.T, entries map[string]byte, name string, wantType byte) {
	t.Helper()

	gotType, ok := entries[name]
	if !ok {
		t.Fatalf("archive missing entry %q", name)
	}
	if gotType != wantType {
		t.Fatalf("archive entry %q type = %v, want %v", name, gotType, wantType)
	}
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	if string(data) != want {
		t.Fatalf("file %q content = %q, want %q", path, string(data), want)
	}
}
