# UI primitives — the brutalist design-system layer

Every visual primitive in the chatz frontend is a real, reusable Svelte 5
component that lives here and is driven **only** by the shared design tokens in
`src/app.css`. New UI is assembled from these building blocks so styling is
identical everywhere.

## The tokens-only rule (hard requirement)

These primitives **never** hardcode a hex color, a px border, or a px
spacing/type value. Color, spacing, borders, type, and the drop-shadow come
exclusively from the `:root` custom properties in `app.css`:

- Color: `--bg --panel --ink --accent --on-accent --muted --border --ok --warn --crit`
- Type: `--font-display --font-mono`, scale `--text-xs … --text-2xl`, `--letter-spacing-label`
- Space: `--space-1 … --space-8`
- Structure: `--border-width` (2px), `--shadow` (hard offset), `border-radius: 0`

A component reaches a semantic color through a **variant** prop (see
`variants.ts`) — never by typing the hex. If you need a new color/size, add a
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
| `Table`      | Brutalist data table from `columns` + `rows`. Reusable by admin lists.                                                                    |
| `Modal`      | Focus-trapped dialog overlay: `title`, `onClose`, `children` body. Escape and scrim click both close; autofocuses the first form control. |
| `StateBlock` | Loading/empty/error scaffold. `variant`: `loading \| empty \| error`. `label`, optional `actions` snippet for dismiss/retry affordances.  |

`variants.ts` holds the variant vocabularies as exported constants — reference
those, never spell the strings inline.

### `rootAttrs` forwarding

`Badge`, `Card`, and `Table` accept an optional `rootAttrs` record spread onto
their outermost element. The json-render catalog components use it to stamp the
`data-jr-type` contract attribute (plus `id`) onto the primitive's root so
generated UI and hand-built UI share the exact same look and markup.

## Reach for these first

Before hand-rolling a button, a bordered box, a status pill, a labeled input,
or a table — **use the primitive here, or extend it**. If none fits, extend an
existing primitive (add a variant) rather than adding a one-off style. Only
create a new primitive when a real duplication exists across the app. Don't
invent unused primitives, and don't add a prop-explosion or a theme engine —
just token-driven reusable components.
