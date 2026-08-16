// Command repogen generates the type-safe gorm/gen repositories from the models
// in internal/pkg/db/models. Run via `make generate`, which drives it through
// the //go:generate directive in the output package's gen.go. The generated
// internal/pkg/db/repositories/*.gen.go files are NEVER hand-edited — change a
// model + re-run.
package main

import (
	"flag"

	"github.com/psyb0t/chatz/internal/pkg/db/models"
	"gorm.io/gen"
)

// defaultOutPath keeps a bare `go run ./cmd/repogen` from the repo root writing
// where it always has. `go generate` runs each directive with the CWD set to
// the directive's own package, so gen.go passes -out=. instead.
const defaultOutPath = "internal/pkg/db/repositories"

func main() {
	outPath := flag.String(
		"out",
		defaultOutPath,
		"directory the generated repositories are written to",
	)

	flag.Parse()

	g := gen.NewGenerator(gen.Config{
		OutPath: *outPath,
		OutFile: "repositories.gen.go",
		Mode: gen.WithoutContext |
			gen.WithDefaultQuery |
			gen.WithQueryInterface,
	})

	g.ApplyBasic(
		models.LLMUsage{},
		models.MCPServer{},
		models.MCPToolExecution{},
		models.Message{},
		models.Project{},
		models.Session{},
		models.User{},
	)

	g.ApplyInterface(func(ChatQuerier) {}, models.Chat{})

	g.Execute()
}
