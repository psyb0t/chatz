# UI primitives: the design-system layer

Every visual primitive in the chatz frontend is a real, reusable Svelte 5
component that lives here and is driven **only** by the shared design tokens in
`src/app.css`. New UI is assembled from these building blocks so styling is
identical everywhere.

## The tokens-only rule (hard requirement)

These primitives **never** hardcode a hex color or a spacing/type value.
Color, spacing, borders, type, radii and shadows come from the `:root` custom
properties in `app.css`, which `:root[data-theme="dark"]` redefines:

- Surfaces: `--bg --panel --panel-2 --surface --code-bg --bubble`
- Text, strongest to weakest: `--ink --muted --faint --disabled`
- Lines and washes: `--border --border-strong --fill-hover --fill-active`
- Accent (links, focus, selection): `--accent --accent-hover --accent-soft --on-accent`
- Primary action (ink on paper): `--primary --primary-hover --on-primary`
- Status: `--ok --warn --crit --ok-soft --crit-soft`
- Type: `--font-sans` (Geist), `--font-mono` (Geist Mono), scale `--text-xs … --text-2xl`, `--text-prose` for message text
- Space: `--space-1 … --space-8`
- Structure: `--border-width` (1px), `--radius-sm … --radius-2xl`, `--radius-pill`, `--control-sm`, `--control`, `--reading-width`
- Elevation and motion: `--shadow-sm --shadow --shadow-lg --shadow-composer`, `--ease --dur-fast --dur`

`Icon.svelte` is the closed inline-SVG icon set (stroke icons on a 24px grid,
drawn in `currentColor`); add a path there rather than pasting SVG into a
component. `Spinner.svelte` is the loading indicator. `app.css` also provides
the `.shimmer` class for live text, the `spin` and `fade-in` keyframes, and
`.sr-only`.

A component reaches a semantic color through a **variant** prop (see
`variants.ts`), never by typing the hex. If you need a new color/size, add a
token to `app.css` first, then reference it.

## Theme and layout safety

Use semantic tokens rather than fixed black/white foregrounds or text-stroke
workarounds; `--ink`, `--muted`, `--panel`, and `--on-accent` change together
with the active theme. Containers that can hold generated content, charts, or
tables must use `min-width: 0` and `max-width: 100%`. Wide data belongs in its
own bounded scroll/clip surface, never in a layout that widens the document.

## The set

| Primitive    | Purpose                                                                                                                                   |
| ------------ | ----------------------------------------------------------------------------------------------------------------------------------------- |
| `Button`     | Actions. `variant`: `default \| primary \| danger`. Primary/danger carry the hard shadow. `type`, `disabled`, `testid`, `onclick`, `id`.  |
| `Badge`      | Semantic status pill. `variant`: `ok \| warn \| crit \| info \| neutral` (neutral = plain bordered).                                      |
| `Chip`       | The plain bordered topbar status pill (`[ADMIN]`, `[MCP:1]`).                                                                             |
| `Card`       | Bordered, shadowed panel with optional `title`/`description` header + `children` body.                                                    |
| `Panel`      | Bordered container block. `shadow` for the hard drop-shadow, `pad`: `md \| lg`.                                                           |
| `Field`      | Labeled form-control wrapper: label + `control` snippet + optional `error`.                                                               |
| `Table`      | Data table from `columns` + `rows`. Reusable by admin lists.                                                                              |
| `Modal`      | Focus-trapped dialog overlay: `title`, `onClose`, `children` body. Escape and scrim click both close; autofocuses the first form control. |
| `StateBlock` | Loading/empty/error scaffold. `variant`: `loading \| empty \| error`. `label`, optional `actions` snippet for dismiss/retry affordances.  |

`variants.ts` holds the variant vocabularies as exported constants. Reference
those, never spell the strings inline.

### `rootAttrs` forwarding

`Badge`, `Card`, and `Table` accept an optional `rootAttrs` record spread onto
their outermost element. The json-render catalog components use it to stamp the
`data-jr-type` contract attribute (plus `id`) onto the primitive's root so
generated UI and hand-built UI share the exact same look and markup.

## Reach for these first

Before hand-rolling a button, a bordered box, a status pill, a labeled input,
or a table: **use the primitive here, or extend it**. If none fits, extend an
existing primitive (add a variant) rather than adding a one-off style. Only
create a new primitive when a real duplication exists across the app. Don't
invent unused primitives, and don't add a prop-explosion or a theme engine.
Just token-driven reusable components.
