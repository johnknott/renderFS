package renderfs

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/flosch/pongo2/v6"
)

func TestCopyBasicRendering(t *testing.T) {
	source := fstest.MapFS{
		"README.md.jinja": {
			Data: []byte("Project: {{ project_name }}\n"),
			Mode: 0o644,
		},
		"src": {
			Mode: fs.ModeDir | 0o755,
		},
		"src/{{ params.app_name }}": {
			Mode: fs.ModeDir | 0o755,
		},
		"src/{{ params.app_name }}/main.go.tmpl": {
			Data: []byte("package {{ params.app_name }}\n"),
			Mode: 0o755,
		},
	}

	dest := t.TempDir()

	context := pongo2.Context{
		"project_name": "RenderFS",
		"params": pongo2.Context{
			"app_name": "demo",
		},
	}

	if err := Copy(source, dest, Options{Context: context}); err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	readme, err := os.ReadFile(filepath.Join(dest, "README.md"))
	if err != nil {
		t.Fatalf("reading rendered README: %v", err)
	}
	if string(readme) != "Project: RenderFS\n" {
		t.Fatalf("unexpected README content: %q", string(readme))
	}

	mainFile := filepath.Join(dest, "src", "demo", "main.go")
	data, err := os.ReadFile(mainFile)
	if err != nil {
		t.Fatalf("reading rendered main.go: %v", err)
	}
	if string(data) != "package demo\n" {
		t.Fatalf("unexpected main.go content: %q", string(data))
	}

	info, err := os.Stat(mainFile)
	if err != nil {
		t.Fatalf("stat rendered file: %v", err)
	}
	if info.Mode()&0o755 != 0o755 {
		t.Fatalf("expected executable permissions, got %v", info.Mode())
	}
}

func TestCopySkipsConditionalPath(t *testing.T) {
	source := fstest.MapFS{
		"{% if params.use_docker %}compose.yaml{% endif %}": {
			Data: []byte("version: '3.8'\n"),
		},
	}
	dest := t.TempDir()
	context := pongo2.Context{
		"params": pongo2.Context{
			"use_docker": false,
		},
	}

	if err := Copy(source, dest, Options{Context: context}); err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dest, "compose.yaml")); !os.IsNotExist(err) {
		t.Fatalf("expected compose.yaml to be skipped, got err=%v", err)
	}
}

func TestCopyRespectsIgnorePatterns(t *testing.T) {
	source := fstest.MapFS{
		".renderfs-ignore": {
			Data: []byte("ignored.txt\n"),
		},
		"kept.txt": {
			Data: []byte("keep me"),
		},
		"ignored.txt": {
			Data: []byte("ignore me"),
		},
	}

	dest := t.TempDir()

	if err := Copy(source, dest, Options{Context: pongo2.Context{}}); err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dest, "ignored.txt")); !os.IsNotExist(err) {
		t.Fatalf("ignored file should not exist (err=%v)", err)
	}

	if _, err := os.Stat(filepath.Join(dest, ".renderfs-ignore")); !os.IsNotExist(err) {
		t.Fatalf(".renderfs-ignore should not be copied (err=%v)", err)
	}

	if _, err := os.Stat(filepath.Join(dest, "kept.txt")); err != nil {
		t.Fatalf("expected kept.txt to exist: %v", err)
	}
}

func TestCopyConflictHandling(t *testing.T) {
	source := fstest.MapFS{
		"file.txt": {
			Data: []byte("new"),
		},
	}

	dest := t.TempDir()
	target := filepath.Join(dest, "file.txt")
	if err := os.WriteFile(target, []byte("original"), 0o644); err != nil {
		t.Fatalf("prepare destination file: %v", err)
	}

	if err := Copy(source, dest, Options{OnConflict: Skip}); err != nil {
		t.Fatalf("Copy with skip failed: %v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read destination file: %v", err)
	}
	if string(data) != "original" {
		t.Fatalf("expected original content preserved, got %q", string(data))
	}

	if err := Copy(source, dest, Options{OnConflict: Fail}); err == nil {
		t.Fatalf("expected failure when OnConflict=Fail")
	}
}

func TestCopyFailsOnMissingVariable(t *testing.T) {
	source := fstest.MapFS{
		"file.txt": {
			Data: []byte("{{ missing }}"),
		},
	}

	dest := t.TempDir()

	err := Copy(source, dest, Options{Context: pongo2.Context{}})
	if err == nil {
		t.Fatalf("expected missing variable error")
	}
}
