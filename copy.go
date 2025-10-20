package renderfs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/flosch/pongo2/v6"
)

// Copy walks the source filesystem, renders templates for paths and file
// contents, and writes the result to destPath.
func Copy(source fs.FS, destPath string, opts Options) error {
	if source == nil {
		return fmt.Errorf("renderfs: source filesystem is required")
	}
	if destPath == "" {
		return fmt.Errorf("renderfs: destination path is required")
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

	destAbs, err := filepath.Abs(destPath)
	if err != nil {
		return fmt.Errorf("renderfs: resolve destination: %w", err)
	}

	if err := createDirectory(destAbs); err != nil {
		return err
	}

	dirModes := make(map[string]fs.FileMode)

	err = fs.WalkDir(source, ".", func(rel string, d fs.DirEntry, walkErr error) error {
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

		destFull, err := resolveDestinationPath(destAbs, renderedRel)
		if err != nil {
			return err
		}

		if d.IsDir() {
			if err := createDirectory(destFull); err != nil {
				return err
			}
			dirModes[destFull] = directoryMode(info)
			return nil
		}

		content, err := fs.ReadFile(source, rel)
		if err != nil {
			return fmt.Errorf("renderfs: read %s: %w", rel, err)
		}

		renderedContent, err := renderTemplateString(string(content), context)
		if err != nil {
			return fmt.Errorf("renderfs: render file %s: %w", rel, err)
		}

		proceed, err := handleConflict(destFull, conflict)
		if err != nil || !proceed {
			return err
		}

		if err := createDirectory(filepath.Dir(destFull)); err != nil {
			return err
		}

		mode := fileMode(info)
		if err := os.WriteFile(destFull, []byte(renderedContent), mode); err != nil {
			return fmt.Errorf("renderfs: write %s: %w", destFull, err)
		}

		if err := os.Chmod(destFull, mode); err != nil {
			return fmt.Errorf("renderfs: chmod %s: %w", destFull, err)
		}

		return nil
	})

	if err != nil {
		return err
	}

	return applyDirectoryModes(dirModes)
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

func resolveDestinationPath(destRoot, renderedRel string) (string, error) {
	if renderedRel == "" {
		return destRoot, nil
	}
	joined := filepath.Join(destRoot, filepath.FromSlash(renderedRel))
	clean, err := filepath.Abs(joined)
	if err != nil {
		return "", fmt.Errorf("renderfs: resolve rendered path: %w", err)
	}

	if clean != destRoot && !strings.HasPrefix(clean, destRoot+string(os.PathSeparator)) {
		return "", fmt.Errorf("renderfs: rendered path %q escapes destination", renderedRel)
	}
	return clean, nil
}

func createDirectory(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("renderfs: create directory %s: %w", dir, err)
	}
	return nil
}

func handleConflict(path string, resolution ConflictResolution) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return true, nil
		}
		return false, fmt.Errorf("renderfs: stat destination %s: %w", path, err)
	}

	if info.IsDir() {
		return false, fmt.Errorf("renderfs: destination %s is a directory", path)
	}

	switch resolution {
	case Skip:
		return false, nil
	case Fail:
		return false, fmt.Errorf("renderfs: destination file %s exists", path)
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

func applyDirectoryModes(modes map[string]fs.FileMode) error {
	if len(modes) == 0 {
		return nil
	}
	paths := make([]string, 0, len(modes))
	for dir := range modes {
		paths = append(paths, dir)
	}
	sort.Slice(paths, func(i, j int) bool {
		return len(paths[i]) > len(paths[j])
	})

	for _, dir := range paths {
		mode := modes[dir]
		if mode == 0 {
			continue
		}
		if err := os.Chmod(dir, mode); err != nil {
			return fmt.Errorf("renderfs: chmod directory %s: %w", dir, err)
		}
	}
	return nil
}
