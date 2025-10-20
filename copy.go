package renderfs

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"

	"github.com/flosch/pongo2/v6"
)

type statWriter interface {
	Lstat(path string) (fs.FileInfo, error)
}

// Copy walks the source filesystem, renders templates for paths and file
// contents, and writes the result to the provided Writer.
func Copy(source fs.FS, dest Writer, opts Options) error {
	if source == nil {
		return fmt.Errorf("renderfs: source filesystem is required")
	}
	if dest == nil {
		return fmt.Errorf("renderfs: destination writer is required")
	}

	context := opts.Context
	if context == nil {
		context = pongo2.Context{}
	}

	conflict := opts.OnConflict
	if conflict < Overwrite || conflict > Fail {
		conflict = Overwrite
	}

	matcher, err := buildIgnoreMatcher(source, opts.IgnorePatterns)
	if err != nil {
		return err
	}

	return fs.WalkDir(source, ".", func(rel string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if rel == "." {
			return nil
		}

		if matcher != nil && matcher.MatchesPath(rel) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		if rel == ".renderfs-ignore" {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("renderfs: stat %s: %w", rel, err)
		}

		renderedRel, skip, err := renderRelativePath(rel, d.IsDir(), context)
		if err != nil {
			return fmt.Errorf("renderfs: render path %s: %w", rel, err)
		}
		if skip {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			return dest.MkdirAll(renderedRel, directoryMode(info))
		}

		if info.Mode()&fs.ModeSymlink != 0 {
			target, err := readSymlink(source, rel)
			if err != nil {
				return fmt.Errorf("renderfs: read symlink %s: %w", rel, err)
			}
			if err := dest.Symlink(target, renderedRel); err != nil {
				return fmt.Errorf("renderfs: create symlink %s -> %s: %w", renderedRel, target, err)
			}
			return nil
		}

		proceed, err := handleConflict(dest, renderedRel, conflict)
		if err != nil {
			return err
		}
		if !proceed {
			return nil
		}

		if parent := path.Dir(renderedRel); parent != "." {
			if err := dest.MkdirAll(parent, 0o755); err != nil {
				return fmt.Errorf("renderfs: create parent %s: %w", parent, err)
			}
		}

		content, err := fs.ReadFile(source, rel)
		if err != nil {
			return fmt.Errorf("renderfs: read %s: %w", rel, err)
		}

		renderedContent, err := renderTemplateString(string(content), context)
		if err != nil {
			return fmt.Errorf("renderfs: render file %s: %w", rel, err)
		}

		handle, err := dest.CreateFile(renderedRel, fileMode(info))
		if err != nil {
			return fmt.Errorf("renderfs: create %s: %w", renderedRel, err)
		}
		if _, err := io.WriteString(handle, renderedContent); err != nil {
			handle.Close()
			return fmt.Errorf("renderfs: write %s: %w", renderedRel, err)
		}
		if err := handle.Close(); err != nil {
			return fmt.Errorf("renderfs: close %s: %w", renderedRel, err)
		}

		return nil
	})
}

func renderRelativePath(rel string, isDir bool, ctx pongo2.Context) (string, bool, error) {
	rendered, err := renderTemplateString(rel, ctx)
	if err != nil {
		return "", false, err
	}

	rendered = strings.TrimSpace(rendered)
	if rendered == "" {
		return "", true, nil
	}

	rendered = strings.ReplaceAll(rendered, "\\", "/")
	clean := path.Clean(rendered)
	if clean == "." {
		return "", true, nil
	}

	if strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") {
		return "", false, fmt.Errorf("renderfs: rendered path %q escapes destination", rendered)
	}

	if !isDir {
		clean = stripTemplateSuffix(clean)
	}

	return clean, false, nil
}

func stripTemplateSuffix(p string) string {
	switch {
	case strings.HasSuffix(p, ".jinja"):
		return strings.TrimSuffix(p, ".jinja")
	case strings.HasSuffix(p, ".tmpl"):
		return strings.TrimSuffix(p, ".tmpl")
	default:
		return p
	}
}

func handleConflict(dest Writer, relPath string, resolution ConflictResolution) (bool, error) {
	sw, ok := dest.(statWriter)
	if !ok {
		if resolution == Skip || resolution == Fail {
			return false, fmt.Errorf("renderfs: destination writer does not support conflict detection for %s", relPath)
		}
		return true, nil
	}

	info, err := sw.Lstat(relPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return true, nil
		}
		return false, fmt.Errorf("renderfs: stat destination %s: %w", relPath, err)
	}

	if info.IsDir() {
		return false, fmt.Errorf("renderfs: destination %s is a directory", relPath)
	}

	switch resolution {
	case Skip:
		return false, nil
	case Fail:
		return false, fmt.Errorf("renderfs: destination file %s exists", relPath)
	default:
		return true, nil
	}
}

func directoryMode(info fs.FileInfo) fs.FileMode {
	perm := fs.FileMode(0o755)
	if info != nil {
		if p := info.Mode().Perm(); p != 0 {
			perm = p
		}
	}
	return perm
}

func fileMode(info fs.FileInfo) fs.FileMode {
	perm := fs.FileMode(0o644)
	if info != nil {
		if p := info.Mode().Perm(); p != 0 {
			perm = p
		}
	}
	return perm
}

func readSymlink(source fs.FS, rel string) (string, error) {
	if rl, ok := source.(fs.ReadLinkFS); ok {
		return rl.ReadLink(rel)
	}
	return "", fmt.Errorf("renderfs: source filesystem does not support symlinks")
}
