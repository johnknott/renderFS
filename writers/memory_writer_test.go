package writers

import (
	"io/fs"
	"testing"
)

func TestMemoryWriterFileLifecycle(t *testing.T) {
	writer := NewMemoryWriter()

	if err := writer.MkdirAll("assets/images", 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if mode, ok := writer.DirMode("assets/images"); !ok || mode != 0o700 {
		t.Fatalf("expected dir mode 700, got %v (ok=%v)", mode, ok)
	}

	handle, err := writer.CreateFile("assets/images/logo.txt", 0o644)
	if err != nil {
		t.Fatalf("CreateFile: %v", err)
	}
	if _, err := handle.Write([]byte("renderfs")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := handle.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	contents := writer.Contents()
	if got := string(contents["assets/images/logo.txt"]); got != "renderfs" {
		t.Fatalf("unexpected content: %q", got)
	}

	if mode, ok := writer.FileMode("assets/images/logo.txt"); !ok || mode != 0o644 {
		t.Fatalf("expected file mode 644, got %v (ok=%v)", mode, ok)
	}

	info, err := writer.Lstat("assets/images/logo.txt")
	if err != nil {
		t.Fatalf("Lstat: %v", err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("expected Lstat perm 644, got %o", info.Mode().Perm())
	}
}

func TestMemoryWriterSymlink(t *testing.T) {
	writer := NewMemoryWriter()

	if err := writer.Symlink("target", "aliases/link"); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	info, err := writer.Lstat("aliases/link")
	if err != nil {
		t.Fatalf("Lstat: %v", err)
	}
	if info.Mode()&fs.ModeSymlink == 0 {
		t.Fatalf("expected symlink mode, got %v", info.Mode())
	}
}
