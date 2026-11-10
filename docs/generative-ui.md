---
title: "Generative UI"
description: "How chatz renders tables, charts, gauges, timelines and log viewers inline from fenced spec blocks in a model reply, and the 26-component catalog."
nav_order: 7
---

# Generative UI

A model reply in chatz can contain live components: tables, status panels, charts, timelines, log viewers and more. The model writes them as JSON Patch lines in a fenced `spec` block, and the web app renders them inline, where they appear in the reply, while the reply streams. Tool results reach the screen the same way: the model reads the data and lays it out.

Every chat's system message carries a generated guide that lists the components and the rules below, so any model that follows instructions can use them. The guide is built from the component catalog in [`web/src/lib/render/`](https://github.com/psyb0t/chatz/tree/main/web/src/lib/render).

## The wire format

The model writes a short explanation, then a ` ```spec ` fence with one [RFC 6902](https://www.rfc-editor.org/rfc/rfc6902) patch per line. The patches build a tree: `/root` names the top element, and each element under `/elements/<key>` has a `type`, `props` and the keys of its `children`.

````text
Error rate on the API tier over the last hour.

```spec
{"op":"add","path":"/elements/rate","value":{"type":"Stat","props":{"id":null,"label":"Error rate","value":"0.4","unit":"%"},"children":[]}}
{"op":"add","path":"/elements/state","value":{"type":"Badge","props":{"id":null,"label":"Healthy","variant":"ok"},"children":[]}}
{"op":"add","path":"/elements/main","value":{"type":"Card","props":{"id":null,"title":"API tier"},"children":["rate","state"]}}
{"op":"add","path":"/root","value":"main"}
```
````

- The element key is its identity. Models set `props.id` to `null`, and the web app stamps the real ID from the key.
- Text and several `spec` fences can mix in one reply; they render in the order written. Patch text is never shown as prose.
- The renderer waits for each patch to be complete JSON before applying it, so a patch split across stream chunks or spread over several lines still works. An open fence renders the part of the tree that is already valid.
- A fence left unterminated reports an error only when the stream ends, and keeps the patches that were valid.

## Components

The catalog is closed: a model cannot use a component that is not on this list, and props are declarative data, with no HTML, callbacks or chart code.

| Group | Components |
| --- | --- |
| Layout and text | `Stack`, `Grid`, `Card`, `Text`, `Heading`, `Callout`, `Badge`, `Progress`, `Timeline`, `KeyValue` |
| Summary and tables | `Table`, `Stat`, `Sparkline` |
| Charts | `TimeSeriesChart`, `AreaChart`, `BarChart`, `DonutChart`, `FunnelChart`, `Gauge`, `ScatterPlot`, `Heatmap`, `Histogram`, `BoxPlot`, `Treemap`, `NetworkGraph` |
| Large data | `LogViewer` |

The props of each component are defined in [`catalog.ts`](https://github.com/psyb0t/chatz/blob/main/web/src/lib/render/catalog.ts), and the exact text models receive is [`genui_instructions.gen.txt`](https://github.com/psyb0t/chatz/blob/main/internal/pkg/core/chats/prompts/genui_instructions.gen.txt).

The guide tells models to use a chart only when it says more than prose or a small table, and to take every value from the conversation or tool results instead of inventing sample data.

## Layout

- Components stay inside the message column. Wide tables and charts scroll inside their own box, never the page.
- Every `Grid` collapses to one column below 40rem, so dashboards reflow on a phone.
- Colors come from the app's theme tokens and follow the light or dark theme.
- The short reveal animation is off when the system asks for reduced motion.

## Seeing it without a model

[Demo mode](chats.md#demo-mode) answers seven fixed prompts with dashboards that use most of the catalog, which is the quickest way to see the components.

## Changing the catalog

Adding or changing a component touches the catalog schema, the Svelte component, the registry and the prompt generator together, then `make genui-prompt` regenerates the guide. The [renderer README](https://github.com/psyb0t/chatz/blob/main/web/src/lib/render/README.md) has the steps and the tests to run.
