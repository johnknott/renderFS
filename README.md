# RenderFS

RenderFS is a lightweight Go library for rendering project templates from any `fs.FS` source onto disk using the [Pongo2](https://github.com/flosch/pongo2) templating engine. It powers templated copies for the Stencil scaffolder while keeping the core rendering logic fast, deterministic, and test friendly. We took inspiration from the excellent [copier](https://github.com/copier-org/copier) project, but wanted something Go-native, dependency-free, and small enough to embed in other tooling.

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

## Examples

### Embedded templates (`//go:embed`)

Most Stencil integrations ship templates inside the binary. RenderFS can work directly with an embedded filesystem:

```go
package main

import (
	"embed"
	"fmt"

	"github.com/flosch/pongo2/v6"
	"github.com/your-org/renderfs"
)

//go:embed templates/**
var templateFS embed.FS

func main() {
	opts := renderfs.Options{
		Context: pongo2.Context{
			"project_name": "My Awesome App",
			"params": pongo2.Context{
				"app_name":   "awesome_app",
				"use_docker": true,
			},
		},
		OnConflict: renderfs.Fail,
	}

	if err := renderfs.Copy(templateFS, "./output", opts); err != nil {
		panic(err)
	}

	fmt.Println("Scaffolding complete!")
}
```

With the following embedded files:

```
templates/
├─ README.md.jinja
├─ src/{{ params.app_name }}/main.go.tmpl
└─ {% if params.use_docker %}compose.yaml{% endif %}
```

The rendered tree will be:

```
output/
├─ README.md
├─ compose.yaml              # Only when params.use_docker == true
└─ src/
   └─ awesome_app/
      └─ main.go
```

### Local directory templates (`os.DirFS`)

You can also render straight from a directory on disk—handy during development before baking templates into the binary:

```go
package main

import (
	"fmt"
	"os"

	"github.com/flosch/pongo2/v6"
	"github.com/your-org/renderfs"
)

func main() {
	source := os.DirFS("./template-src")

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

The directory `template-src` can then be committed alongside your project and exercised or updated without re-building the binary.

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
