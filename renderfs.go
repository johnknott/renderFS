package renderfs

import (
	"io"
	"io/fs"

	"github.com/flosch/pongo2/v6"
)

// ConflictResolution defines how Copy should behave when a destination file already exists.
type ConflictResolution int

const (
	// Overwrite replaces any existing file at the destination.
	Overwrite ConflictResolution = iota
	// Skip leaves an existing file untouched.
	Skip
	// Fail aborts the copy operation when a destination file exists.
	Fail
)

// Options configures the behaviour of the Copy operation.
type Options struct {
	// Context provides template data when rendering path and file contents.
	// When nil, an empty context is used.
	Context pongo2.Context

	// OnConflict controls how Copy reacts when the destination file already exists.
	// Defaults to Overwrite when left zero-valued.
	OnConflict ConflictResolution

	// IgnorePatterns contains gitignore-style patterns that should be excluded
	// from the copy. When empty, Copy looks for a .renderfs-ignore file at the
	// root of the source filesystem.
	IgnorePatterns []string
}

// Writer abstracts the destination that rendered files and directories are
// written to. Implementations can target the local filesystem, in-memory
// stores, archives, or any other medium.
type Writer interface {
	// MkdirAll creates the directory tree at path (relative to the writer's
	// root) with the provided permissions.
	MkdirAll(path string, perm fs.FileMode) error

	// CreateFile opens or creates the file at path for writing with the given
	// permission bits, truncating any existing content.
	CreateFile(path string, perm fs.FileMode) (io.WriteCloser, error)

	// Symlink creates a symbolic link named newname pointing to oldname. Writers
	// that do not support symlinks should return an error such as fs.ErrInvalid.
	Symlink(oldname, newname string) error
}
