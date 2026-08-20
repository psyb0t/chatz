# Generative UI renderer

`web/src/lib/render` transforms assistant text containing fenced ` ```spec `
blocks into inline Svelte UI. The backend streams original text; this module
owns fence parsing, JSON Patch assembly, catalog validation, registration, id
stamping, and responsive rendering.

## Contents

- [Wire and stream contract](#wire-and-stream-contract)
- [Catalog and layout](#catalog-and-layout)
- [Safe change workflow](#safe-change-workflow)
- [Verification](#verification)

## Wire and stream contract

Models place RFC-6902 patch objects in a ` ```spec ` fence. The element key
under `/elements/<key>` is the stable identity; models use `props.id: null` and
`stamp.ts` assigns the rendered id/contract attribute. Prose and multiple spec
fences retain their original order, and raw patch text is never shown as prose.

`fence.ts` buffers SSE/fence boundaries and waits for structurally complete JSON
before applying a patch. It supports pretty-printed multi-line patch JSON, not
only one compact patch per line. An open fence renders its valid partial tree;
an unterminated patch reports an error only at true stream end, without
discarding earlier valid patches.

`AssistantContent.svelte` re-derives segments from the growing message and
deduplicates render telemetry. There is no free-floating “Generating …” label:
an element appears at its natural inline location once its tree exists.

## Catalog and layout

The closed typed catalog has 26 components:

| Group         | Components                                                                                                                                                    |
| ------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Layout/text   | `Stack`, `Grid`, `Card`, `Text`, `Heading`, `Callout`, `Badge`, `Progress`, `Timeline`, `KeyValue`                                                            |
| Summary/table | `Table`, `Stat`, `Sparkline`                                                                                                                                  |
| Charts        | `TimeSeriesChart`, `AreaChart`, `BarChart`, `DonutChart`, `FunnelChart`, `Gauge`, `ScatterPlot`, `Heatmap`, `Histogram`, `BoxPlot`, `Treemap`, `NetworkGraph` |
| Large data    | `LogViewer`                                                                                                                                                   |

Models cannot choose unregistered components or executable chart options. The
embedded guide requires values to come from conversation/tool data. Showcase
mode (`make run-showcase`) demonstrates the catalog through paced synthetic
operations, sales, customer-risk, and other business investigations.

Renderer wrappers use `min-width: 0` and `max-width: 100%`. Charts/tables stay
inside their message column; local overflow is bounded to the component, never
the document. Every model-requested `Grid` collapses to one column at 40rem so
dashboard panels reflow rather than squeeze at phone width. Semantic tokens
from `web/src/app.css` support both themes. The short scale/fade reveal is
disabled by `prefers-reduced-motion: reduce`.

## Safe change workflow

These representations must remain lockstepped:

```text
catalog.ts + common/render.ts + registry.ts
          ↓
web/scripts/gen-ui-instructions.mjs
          ↓ make genui-prompt
internal/pkg/core/chats/prompts/genui_instructions.gen.txt
```

When changing a component, update its catalog schema/description, implementation
under `components/`, shared name, registry entry, and generator mirror. Run
`make genui-prompt`; never hand-edit the generated `.gen.txt`. Extend catalog,
fence, and renderer tests plus a showcase fixture when a new behavior needs one.
Keep
props declarative and bounded: no arbitrary HTML, executable callbacks, or
side-effecting URLs.

## Verification

| Path                           | Responsibility                      |
| ------------------------------ | ----------------------------------- |
| `catalog.ts` / `registry.ts`   | Schema contract and Svelte binding. |
| `fence.ts`                     | Ordered parsing and patch assembly. |
| `Renderer.svelte` / `stamp.ts` | Provider and stable ids.            |
| `components/` / `charts/`      | Typed UI and chart helpers.         |

Run `make genui-prompt`, `make web-check`, `make web-test`,
`make web-format-check`, and `make test-api` (the showcase render is covered by
the Go browser driver `tests/api/showcase_test.go`).
