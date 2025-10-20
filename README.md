# RenderFS

RenderFS is a lightweight Go library for rendering project templates from any `fs.FS` source onto disk using the [Pongo2](https://github.com/flosch/pongo2) templating engine. It powers templated copies for the Stencil scaffolder while keeping the core rendering logic fast, deterministic, and test friendly.

## Features

- Render both file *paths* and file *contents* with Pongo2 templates.
- Support `.jinja` and `.tmpl` suffix stripping after rendering.
- Conditional file and directory creation (empty rendered paths are skipped).
- `.renderfs-ignore` (or explicit patterns) using gitignore semantics.
- Preserve source file permissions, including executable bits.
- Conflict handling modes: overwrite, skip, or fail fast.

## Installation

```bash
go get github.com/your-org/renderfs
```

## Example

```go
package main

import (
	"fmt"
	"io/fs"
	"testing/fstest"

	"github.com/flosch/pongo2/v6"
	"github.com/your-org/renderfs"
)

func main() {
	source := fstest.MapFS{
		"README.md.jinja": {
			Data: []byte("Project: {{ project_name }}\n"),
		},
		"src/{{ params.app_name }}/main.go.tmpl": {
			Data: []byte("package {{ params.app_name }}\n"),
		},
		"{% if params.use_docker %}compose.yaml{% endif %}": {
			Data: []byte("version: '3.8'\n"),
		},
	}

	opts := renderfs.Options{
		Context: pongo2.Context{
			"project_name": "My Awesome App",
			"params": pongo2.Context{
				"app_name":   "awesome_app",
				"use_docker": true,
			},
		},
		OnConflict: renderfs.Overwrite,
	}

	if err := renderfs.Copy(source, "./output", opts); err != nil {
		panic(err)
	}

	fmt.Println("Scaffolding complete!")
}
```

Running the example will create the following files:

```
output/
├─ README.md                   # Rendered from README.md.jinja
└─ src/
   └─ awesome_app/
      └─ main.go               # Rendered from main.go.tmpl
```

`compose.yaml` is only created when `params.use_docker` is truthy.

## Ignore Patterns

RenderFS honours gitignore-style patterns in either:

- `Options.IgnorePatterns` (takes precedence), or
- a `.renderfs-ignore` file located at the root of the source filesystem.

Ignored files/directories are skipped during the copy, and `.renderfs-ignore` itself is never written to the destination.

## Conflict Handling

`Options.OnConflict` controls what happens when a destination file already exists:

| Mode       | Behaviour                                               |
|------------|---------------------------------------------------------|
| Overwrite  | Replace the existing file (default).                    |
| Skip       | Leave the existing file untouched.                      |
| Fail       | Abort the copy and return an error immediately.         |

## Development

Tooling is managed via [mise](https://github.com/jdx/mise). The project ships with convenient tasks:

```bash
mise run fmt    # go fmt ./...
mise run test   # go test ./...
mise run build  # go build ./...
```

## License

MIT © Your Org

