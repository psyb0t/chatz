# goenv

Instructions for coding agents (Codex, Claude Code and anything else that reads `AGENTS.md`) working on this repository, and for the humans reviewing their changes.

## Agent setup

`CLAUDE.md` is a committed symlink to `AGENTS.md`, and `.claude` is a committed symlink to `.agents`, so Claude Code reads the same instructions, rules and skills as Codex. Claude's Edit and Write tools refuse to write through a symlink, so edit `AGENTS.md` and `.agents/rules/*.md` directly.

## What it is

`github.com/psyb0t/goenv` is a single-file Go library that reads the `ENV` environment variable and reports whether the process runs in `dev` or `prod`. `Get()` returns `Dev` only when `ENV` is exactly `"dev"` and `Prod` for every other value, unset and empty included. The README plays it as a joke ("the most over-engineered environment variable reader"); the code is one `switch` on one variable and must stay that small.

## Architecture

There is one package, `goenv`, in one file, `goenv.go`:

- `EnvVarName = "ENV"`: the variable it reads.
- `type Type = string`: a type alias, not a defined type, so `Type` and `string` are interchangeable.
- `Prod` and `Dev`: the two `Type` constants, `"prod"` and `"dev"`.
- `Get() Type`: calls `os.Getenv(EnvVarName)` on every call (no caching) and switches on the exact, case-sensitive value.
- `IsProd() bool` and `IsDev() bool`: `Get() == Prod` and `Get() == Dev`.

The only import is the standard library `os`. There is no state, no init and no error path.

## Stack

- Go 1.26 (`go 1.26` in `go.mod`, `go_version: "1.26"` in CI). No third-party modules, so there is no `go.sum`.
- Make for every command. Lint is `go fix -diff` plus `go vet`; there is no golangci-lint config here.
- GitHub Actions through the shared workflows in [psyb0t/reusable-github-workflows](https://github.com/psyb0t/reusable-github-workflows).
- Docs: Jekyll with the just-the-docs theme, built by GitHub Pages from `docs/` on `main`.

## Layout

| Path | Holds |
| --- | --- |
| `goenv.go` | The whole library. |
| `goenv_test.go` | Table tests for `Get`, `IsProd` and `IsDev`, using `t.Setenv`. |
| `Makefile` | `dep`, `lint`, `lint-fix`, `test`, `test-coverage`, `docs`, `help`. |
| `pre-commit.sh` | `make lint && make test-coverage`. |
| `docs/` | The documentation site, published at https://psyb0t.github.io/goenv/. |
| `.agents/skills/goenv/` | The agent skill (`SKILL.md`, `references/setup.md`), published to ClawHub on tags. |
| `.agents/.claude-plugin/`, `.agents/.codex-plugin/` | The Claude Code and Codex plugin manifests. |
| `.agents/rules/` | The rules below. |
| `.github/workflows/pipeline.yml` | Lint, tests, govulncheck, badges, release and the ClawHub publish. |
| `.github/workflows/mirror-and-archive.yml`, `issue-pull.yml`, `collaborators-only.yml` | Mirroring, archiving, issue sync and the no-PR policy. Not part of the build. |
| `CHANGELOG.md` | One section per release tag. |

## Commands

Go 1.26 and Make are enough on the host. Without a local Go 1.26, run the same targets in the official image:

```bash
docker run --rm \
  --user "$(id -u):$(id -g)" \
  -e HOME=/tmp \
  -e GOCACHE=/tmp/gocache \
  -e GOPATH=/tmp/go \
  -v "$PWD:/src" \
  -w /src \
  golang:1.26 \
  make dep lint test-coverage
```

```bash
make help            # every target
make dep             # go mod tidy
make lint            # go fix -diff (fails on any suggestion), then go vet
make lint-fix        # go fix, then go vet
make test            # go test -race ./...
make test-coverage   # race tests with coverage, writes coverage-percent.txt, fails below 90%
make docs            # build the docs site into .testing/docs-site with GitHub's Jekyll image
```

`make docs` needs Docker. What to check in the built site is in `.agents/rules/docs-and-releases.md`.

## Rules

Rules for every change live in `.agents/rules/`, one file per topic. Read the ones that match what you touch before editing.
