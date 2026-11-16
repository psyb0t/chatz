# gonfiguration 🔧

[![Go Reference](https://pkg.go.dev/badge/github.com/psyb0t/gonfiguration.svg)](https://pkg.go.dev/github.com/psyb0t/gonfiguration)
[![CI](https://github.com/psyb0t/gonfiguration/actions/workflows/pipeline.yml/badge.svg?branch=master)](https://github.com/psyb0t/gonfiguration/actions/workflows/pipeline.yml)
[![coverage](https://raw.githubusercontent.com/psyb0t/gonfiguration/badges/coverage.svg)](https://github.com/psyb0t/gonfiguration/actions/workflows/pipeline.yml)
[![version](https://raw.githubusercontent.com/psyb0t/gonfiguration/badges/version.svg)](https://github.com/psyb0t/gonfiguration/tags)
[![license](https://raw.githubusercontent.com/psyb0t/gonfiguration/badges/license.svg)](https://github.com/psyb0t/gonfiguration/blob/master/LICENSE)
[![imported by](https://raw.githubusercontent.com/psyb0t/gonfiguration/badges/importers.svg)](https://github.com/psyb0t/gonfiguration/blob/badges/importers.md)

A no-bullshit, thread-safe configuration library for Go that doesn't make you wanna punch your monitor. Tired of writing the same boring-ass env var parsing shit over and over? This badass package's got your back with reflection magic that actually works without making you cry.

## What this beast can do

This ain't your granddad's config parser.

### Supported types

- `string` and `bool`, the bread and butter
- `int`, `int8`, `int16`, `int32`, `int64`, all the flavors you need
- `uint`, `uint8`, `uint16`, `uint32`, `uint64`, for when you don't do negative vibes
- `float32` and `float64`, because math is hard
- `time.Duration`, parsed with Go's native format (`"5s"`, `"10m"`, `"1h30m"`)
- `[]string`, comma-separated and split for you, spaces trimmed (`"val1, val2,val3"`)

### Core features

- **Thread-safe**: concurrent `Parse` and `SetDefault` calls won't shit the bed. The state sits behind a `sync.RWMutex`.
- **Default values**: set fallbacks with a `default` struct tag or from code, so your app doesn't break when someone forgets to set an env var.
- **Required fields**: mark a field `required` and `Parse` errors out when nothing sets it.
- **Errors that tell you where**: every error carries the file, line and function of each hop it went through via [ctxerrors](https://github.com/psyb0t/ctxerrors), and `errors.Is()` still matches the sentinels underneath.
- **Type checks**: a value that doesn't parse for its field is an error, not a silent zero.

## Installation

```bash
go get github.com/psyb0t/gonfiguration
```

Needs Go 1.26 or newer.

## Basic usage example

```go
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/psyb0t/gonfiguration"
)

type AppConfig struct {
	// Defaults right in the struct
	ListenAddress string        `env:"LISTEN_ADDRESS" default:"127.0.0.1:8080"`
	Debug         bool          `env:"DEBUG" default:"false"`
	Timeout       time.Duration `env:"TIMEOUT" default:"30s"`
	AllowedHosts  []string      `env:"ALLOWED_HOSTS" default:"localhost,127.0.0.1"`

	// Required fields error out if nothing sets them
	DBName string `env:"DB_NAME,required"`
	DBUser string `env:"DB_USER,required"`
	DBPass string `env:"DB_PASS,required"`
}

func main() {
	cfg := AppConfig{}

	// Parse that shit
	if err := gonfiguration.Parse(&cfg); err != nil {
		log.Fatalf("holy fuque! config parsing failed: %v", err)
	}

	fmt.Printf("Allowed hosts: %v\n", cfg.AllowedHosts)
	fmt.Printf("Timeout: %v\n", cfg.Timeout)
}
```

Run it with `DB_NAME`, `DB_USER` and `DB_PASS` set and you get the defaults for everything else. Set `ALLOWED_HOSTS="api.example.com, cdn.example.com"` and the slice comes out split and trimmed. Leave `DB_NAME` out and `Parse` returns an error matching `gonfiguration.ErrRequiredFieldNotSet`.

Want fallbacks from code instead of tags? `gonfiguration.SetDefault("TIMEOUT", 30*time.Second)` beats the tag, and an env var beats both. The full precedence rules are in [Defaults and precedence](https://psyb0t.github.io/gonfiguration/defaults/).

## Rules and limitations (read this shit)

1. **No `env` tag, no parsing.** Untagged fields are skipped.
2. **Pass a pointer to `Parse()`**, not the struct itself, you savage.
3. **Flat structs only.** Nested structs, pointers and maps fail with `ErrUnsupportedFieldType`.
4. **Programmatic defaults must match the field type exactly.** `8080` is an `int`, so an `int64` field needs `int64(8080)`.
5. **Narrow ints wrap silently.** `300` in an `int8` field becomes `44`, no error. Check ranges yourself.
6. **Set but empty still counts as set.** `PORT=` overrides every default and then fails to parse as an int.
7. **Parse errors quote the bad value**, so don't spray them into logs if the variable holds a secret.
8. **`Reset()` is for tests.** It's not safe to call while other goroutines are using the library.

## Documentation

The full docs live at **[psyb0t.github.io/gonfiguration](https://psyb0t.github.io/gonfiguration/)**.

| Page | What's in it |
| --- | --- |
| [Struct tags and types](https://psyb0t.github.io/gonfiguration/types/) | The `env`, `required` and `default` tags, every supported type and how each is parsed |
| [Defaults and precedence](https://psyb0t.github.io/gonfiguration/defaults/) | `default` tags, `SetDefault`/`SetDefaults`, which value wins, the type rule |
| [Errors](https://psyb0t.github.io/gonfiguration/errors/) | Every sentinel, when you get it, and what the wrapped messages look like |
| [API reference](https://psyb0t.github.io/gonfiguration/api/) | Every exported function, the global state, and what's safe across goroutines |
| [Development](https://psyb0t.github.io/gonfiguration/development/) | Make targets, the coverage gate, CI and releases |

A longer runnable example is in [`.example/main.go`](https://github.com/psyb0t/gonfiguration/blob/master/.example/main.go), and the changes per release are in the [CHANGELOG](https://github.com/psyb0t/gonfiguration/blob/master/CHANGELOG.md).

## Sponsor

<a href="https://www.buymeacoffee.com/psyb0t"><img src="https://cdn.buymeacoffee.com/buttons/v2/default-blue.png" alt="Buy Me a Coffee" height="60" width="217"></a>

## License

Copyright 2023-2026 Ciprian Mandache ([ciprian.51k.eu](https://ciprian.51k.eu))

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
