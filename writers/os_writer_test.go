package writers

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOSWriterCreatesDirectoriesAndFiles(t *testing.T) {
	dest := t.TempDir()
	writer, err := NewOSWriter(dest)
	if err != nil {
		t.Fatalf("NewOSWriter: %v", err)
	}

	if err := writer.MkdirAll("nested/dir", 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	dirInfo, err := os.Stat(filepath.Join(dest, "nested/dir"))
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if perm := dirInfo.Mode().Perm(); perm != 0o750 {
		t.Fatalf("expected dir perm 750, got %o", perm)
	}

	handle, err := writer.CreateFile("nested/dir/file.txt", 0o644)
	if err != nil {
		t.Fatalf("CreateFile: %v", err)
	}
	if _, err := handle.Write([]byte("hello")); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := handle.Close(); err != nil {
		t.Fatalf("close file: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dest, "nested/dir/file.txt"))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(content) != "hello" {
		t.Fatalf("unexpected file content: %q", string(content))
	}

	fileInfo, err := writer.Lstat("nested/dir/file.txt")
	if err != nil {
		t.Fatalf("Lstat: %v", err)
	}
	if fileInfo.Mode().Perm() != 0o644 {
		t.Fatalf("expected file perm 644, got %o", fileInfo.Mode().Perm())
	}
}

func TestOSWriterSymlink(t *testing.T) {
	dest := t.TempDir()
	writer, err := NewOSWriter(dest)
	if err != nil {
		t.Fatalf("NewOSWriter: %v", err)
	}

	if err := writer.Symlink("target.txt", "link.txt"); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	target, err := os.Readlink(filepath.Join(dest, "link.txt"))
	if err != nil {
		t.Fatalf("Readlink: %v", err)
	}
	if target != "target.txt" {
		t.Fatalf("unexpected link target: %q", target)
	}
}
