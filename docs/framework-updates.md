---
title: "Framework updates"
description: "How make servicepack-update brings framework fixes into chatz: the update branch, what it preserves, the ignore file, backups and the checks to run after."
nav_order: 1
parent: "Development"
---

# Framework updates

A project created from Servicepack owns its services and application-specific
extension points, while retaining a copy of the reusable framework. Framework
updates bring the latter forward without blindly replacing the former.

## Before updating

Start with a clean downstream repository. The update bootstrap refuses to run
when there are uncommitted changes, because it needs to make a reviewable
update branch and distinguish update changes from yours.

```bash
make servicepack-update
```

The bootstrap resolves the latest Servicepack tag (falling back to the current
main revision), downloads a fresh copy, and then executes that downloaded
copy's update logic. That means a release can fix update policy on the same
run that installs it rather than being trapped behind an old local updater.

The apply phase:

1. creates a backup;
2. creates `servicepack_update_to_<version>` from your current branch;
3. syncs framework-owned files onto that branch;
4. preserves your module path and merges framework dependency floors without
   wholesale replacement of your `go.mod`; and
5. commits the synchronized update on that branch, then leaves it checked out
   for review/merge or reversion through the matching Make targets.

Use the exact review and disposition commands exposed by your current template:

```bash
make servicepack-update-review
make servicepack-update-merge
make servicepack-update-revert
```

The update machinery uses Git internally because it is making a branch and
backup in your project. Read its output and review the branch like any other
framework upgrade before treating it as deployable.

The implementation and per-target override rules live in the
[framework Make-script README](https://github.com/psyb0t/chatz/blob/main/scripts/make/servicepack/README.md).

## What is preserved

The framework always excludes application material such as:

- `internal/pkg/services/*`;
- the `docs/` and `tests/` trees;
- root `README.md`, `LICENSE`, `CHANGELOG.md`, `.gitignore`, `Makefile`, and
  project Dockerfiles;
- your `go.mod`, `go.sum`, and vendored graph, which receive dependency-aware
  handling instead of a blind file replacement;
- build artifacts and Git internals.

chatz's [`.servicepackupdateignore`](https://github.com/psyb0t/chatz/blob/main/.servicepackupdateignore) adds its own extension points and customized files: `cmd/init.go`, `cmd/commands.go`, `.golangci.yml`, the Go dependency age-gate script, the Docker build context, the secret-scanning allowlist, Dependabot, the CI pipeline, the funding links, the `issue-pull` and `mirror-and-archive` workflows, the agent rules in `.agents`, and the whole `tests/` tree.

That ignore file belongs to the project. New exclusions added upstream later are not forced into an existing project, so compare the new template when you update if you want its newer defaults.

## Customize the boundary deliberately

If you truly need to keep a framework-owned file unchanged, add its path to
`.servicepackupdateignore` before updating. That is an opt-out with a cost:
you now own merging future fixes in that file. Prefer public extension points
first:

- lifecycle hooks in `cmd/init.go`;
- app commands in `cmd/commands.go`;
- Make and script overrides in `Makefile` and `scripts/make/`;
- project `Dockerfile` / `Dockerfile.dev`.

Do not use a broad ignore pattern just to make an update look clean. A clean
diff achieved by skipping framework changes is not an update.

## Version tracking and backups

`make own` creates `servicepack.version` when the new project lacks one. It
records the framework revision from which the project began; the updater uses
it to decide whether a newer framework version exists.

Backups are created before an update. The template also exposes:

```bash
make backup
make backup-restore
make backup-clear
```

Restore and clear operations alter project or backup data; read their target
help and verify the selected backup before using them.

## After updating

Review the update branch, resolve deliberate project/framework interactions,
then run the relevant Docker-backed checks from [development](development.md):

```bash
make test
make lint
make test-coverage
```

If the update changes lifecycle behavior, also exercise a real startup and
shutdown path. The [architecture overview](architecture.md) identifies the
framework-owned areas most likely to affect that behavior.
