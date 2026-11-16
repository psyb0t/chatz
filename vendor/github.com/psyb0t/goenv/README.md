# goenv

[![Go Reference](https://pkg.go.dev/badge/github.com/psyb0t/goenv.svg)](https://pkg.go.dev/github.com/psyb0t/goenv)
[![CI](https://github.com/psyb0t/goenv/actions/workflows/pipeline.yml/badge.svg?branch=main)](https://github.com/psyb0t/goenv/actions/workflows/pipeline.yml)
[![coverage](https://raw.githubusercontent.com/psyb0t/goenv/badges/coverage.svg)](https://github.com/psyb0t/goenv/actions/workflows/pipeline.yml)
[![version](https://raw.githubusercontent.com/psyb0t/goenv/badges/version.svg)](https://github.com/psyb0t/goenv/tags)
[![license](https://raw.githubusercontent.com/psyb0t/goenv/badges/license.svg)](https://github.com/psyb0t/goenv/blob/main/LICENSE)
[![imported by](https://raw.githubusercontent.com/psyb0t/goenv/badges/importers.svg)](https://github.com/psyb0t/goenv/blob/badges/importers.md)

The most over-engineered environment variable reader in the history of Go programming. A full-blown, battle-tested, enterprise-grade package that reads ONE environment variable and tells you if you're in prod or dev.

## Why does this exist?

Because `os.Getenv("ENV")` is 16 characters and sometimes you just need a whole fucking package to do the same thing but with ✨style✨.

## Features

- Reads the `ENV` environment variable. That's it. That's the feature.
- Defaults to `prod` because production is where dreams go to die and that's where you probably are.
- Can tell you if you're in `dev`. Revolutionary.
- Zero dependencies. Not even `fmt`. I don't need that shit.
- 100% test coverage because I'm a professional.

## Install

```bash
go get github.com/psyb0t/goenv
```

Needs Go 1.26 or newer.

## Usage

```go
package main

import (
    "fmt"

    "github.com/psyb0t/goenv"
)

func main() {
    // Get the current environment
    env := goenv.Get() // "prod" or "dev"
    fmt.Println("running in", env)

    // Check if prod
    if goenv.IsProd() {
        fmt.Println("don't fuck this up")
    }

    // Check if dev
    if goenv.IsDev() {
        fmt.Println("break whatever you want")
    }
}
```

## Environment variable

```bash
export ENV=dev   # you're developing
export ENV=prod  # you're in production
export ENV=      # also production, because paranoia is a feature
```

Only the exact, lowercase `dev` counts as dev. `DEV`, `development`, `staging`, a typo: all prod. The full table of values, every function and constant, and the `Type` alias are in the [API reference](https://psyb0t.github.io/goenv/api/).

## Agent integrations

The [skill](https://github.com/psyb0t/goenv/tree/main/.agents/skills/goenv) works in any agent that reads `.agents/skills/` (such as [peen](https://github.com/psyb0t/peen)), installs in Claude Code and Codex from the `psyb0t/agents` marketplace, and lands on ClawHub for OpenClaw on every release. The install commands are on [Agent integrations](https://psyb0t.github.io/goenv/agent-integrations/).

## Documentation

The full docs live at **[psyb0t.github.io/goenv](https://psyb0t.github.io/goenv/)**.

| Page | What's in it |
| --- | --- |
| [API reference](https://psyb0t.github.io/goenv/api/) | `Get`, `IsProd`, `IsDev`, the constants, the `Type` alias and which `ENV` values mean dev |
| [Agent integrations](https://psyb0t.github.io/goenv/agent-integrations/) | Installing the skill in Claude Code, Codex and OpenClaw |
| [Development](https://psyb0t.github.io/goenv/development/) | Make targets, the coverage gate, CI and releases |

## FAQ

**Q: Do I really need a package for this?**
A: No. But here we are.

**Q: What if I set ENV to "staging"?**
A: You get prod. Because if it's not explicitly dev, it's prod. I don't trust you.

**Q: What about ENV=test?**
A: Also prod. See above.

**Q: Is this thread-safe?**
A: It calls `os.Getenv()`. Take that however you want.

**Q: Why not just use os.Getenv directly?**
A: Because then you'd have to remember the variable name, write a switch statement, handle the default case, and before you know it you've written this package anyway.

## Sponsor

<a href="https://www.buymeacoffee.com/psyb0t"><img src="https://cdn.buymeacoffee.com/buttons/v2/default-blue.png" alt="Buy Me a Coffee" height="60" width="217"></a>

## License

MIT. Free as in "do whatever the hell you want with it". Which, given what this package does, isn't saying much.
