# gonfiguration

Instructions for coding agents (Claude Code, Codex and the rest) and for people working on this repository. Users of the library want the [documentation site](https://psyb0t.github.io/gonfiguration/) instead.

## Agent setup

`CLAUDE.md` is a committed symlink to `AGENTS.md`, and `.claude` is a committed symlink to `.agents`, so Claude Code reads the same instructions, rules and skills as Codex. Claude's Edit and Write tools refuse to write through a symlink, so edit `AGENTS.md` and `.agents/rules/*.md` directly.

## What it is

A Go library, `github.com/psyb0t/gonfiguration`, that fills a struct from environment variables using reflection. A field opts in with an `env:"NAME"` tag, can add `,required`, and can carry a `default:"..."` tag. Defaults can also be set from code with `SetDefault` and `SetDefaults`. Errors are wrapped with `github.com/psyb0t/ctxerrors`; apart from that the library imports only the standard library. No binary, no Docker image.

The library is imported by more packages than anything else in the owner's fleet (the imported-by badge counts them), so a change to an exported name or to parsing behavior breaks other people's builds.

## Architecture

One process-wide `*gonfiguration` value, `gonfig`, created in `init()` and guarded by its embedded `sync.RWMutex`. It holds two maps:

- `defaults` (`map[string]any`): values set with `SetDefault` and `SetDefaults`.
- `envVars` (`map[string]string`): every variable from `os.Environ()`, cached by `Parse`. Entries are added or overwritten on each `Parse`, never removed, until `Reset`.

`Parse(dst)` flow: read `os.Environ()` and cache it, check that `dst` is a non-nil pointer to a struct, then for each field with an `env` tag: check the type with `isSupportedType`, apply the `default` tag, then a programmatic default (its dynamic type must equal the field type), then the environment variable. `required` fails only when none of the three supplied a value. Strings are converted by `setEnvVarValue` and the `set*` helpers.

## Stack

- Go: `go 1.26` in `go.mod`; CI runs Go 1.26 through the shared `go-workflow.yml`.
- Runtime dependency: `github.com/psyb0t/ctxerrors`. Tests use `github.com/stretchr/testify`.
- Lint: golangci-lint v2, pinned as a `tool` in `go.mod`, run with `go tool golangci-lint` and configured in `.golangci.yml`.
- Dependencies are vendored in `vendor/` (`make dep` regenerates it).
- Docs: Jekyll with the just-the-docs theme, built by GitHub Pages from `docs/` on `master`.

## Layout

| Path | Holds |
| --- | --- |
| `gonfiguration.go` | `Parse`, `MustParse`, `GetAllValues`, `Reset`, the global state, tag parsing and the per-type setters. |
| `defaults.go` | `SetDefault`, `SetDefaults`, `GetDefaults` and the locked map access. |
| `env_vars.go` | `GetEnvVars`, the env var cache and the `os.Environ()` reader. |
| `errors.go` | The seven exported sentinel errors. |
| `gonfiguration_test.go` | Black-box tests (`package gonfiguration_test`). |
| `gonfiguration_internal_test.go` | White-box tests of unexported helpers (`package gonfiguration`). |
| `.example/main.go` | A runnable example. The leading dot keeps it out of `./...`, so it is not linted or tested. |
| `docs/` | The documentation site, published at https://psyb0t.github.io/gonfiguration/. |
| `.github/workflows/pipeline.yml` | CI: lint, tests, govulncheck, releases from tags, badges. |
| `vendor/` | Vendored modules. Never edit by hand. |

## Commands

Go 1.26 and Make, or Docker with the `golang:1.26` image. Run `make help` for the list.

```bash
make all              # dep, lint-fix and test-coverage, in that order
make dep              # go mod tidy && go mod vendor
make lint             # fails on go fix -diff output, then golangci-lint
make lint-fix         # go fix and golangci-lint --fix
make test             # go test -race ./...
make test-coverage    # tests with a coverage profile, fails below 90%
make docs             # build the docs site into .testing/docs-site (needs Docker)
```

Run one test with `go test -race -run TestName ./...`. `pre-commit.sh` runs `make lint && make test-coverage`, the same pair CI runs.

Without a local Go 1.26, run the targets in a container:

```bash
docker run --rm \
  --user "$(id -u):$(id -g)" \
  -e HOME=/tmp \
  -e GOCACHE=/repo/.testing/gocache \
  -v "$PWD:/repo" \
  -w /repo \
  golang:1.26 \
  make lint test-coverage
```

## Rules

Rules for every change are in `.agents/rules/`. Read the ones that cover what you are touching before editing.
