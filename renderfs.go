package renderfs

import "github.com/flosch/pongo2/v6"

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
