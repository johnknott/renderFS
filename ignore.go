package renderfs

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	ignore "github.com/sabhiram/go-gitignore"
)

func buildIgnoreMatcher(source fs.FS, patterns []string) (*ignore.GitIgnore, error) {
	lines := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		lines = append(lines, pattern)
	}

	if len(lines) == 0 {
		raw, err := fs.ReadFile(source, ".renderfs-ignore")
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("renderfs: read .renderfs-ignore: %w", err)
		}
		if len(raw) > 0 {
			lines = append(lines, parseIgnoreFile(string(raw))...)
		}
	}

	if len(lines) == 0 {
		return nil, nil
	}

	lines = append(lines, ".renderfs-ignore")
	return ignore.CompileIgnoreLines(lines...), nil
}

func parseIgnoreFile(content string) []string {
	var patterns []string
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}
	return patterns
}
