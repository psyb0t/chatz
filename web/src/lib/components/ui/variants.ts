// Shared variant vocabularies for the brutalist UI primitives. Kept out of the
// .svelte files so callers reference the same strings the components switch on
// (never spelled inline). Each variant maps to a semantic color token in
// app.css (--ok / --warn / --crit / --accent) — a variant is the ONLY knob a
// caller turns to reach a token color; hex/px never appear in component markup.

export const BUTTON_DEFAULT = "default";
export const BUTTON_PRIMARY = "primary";
export const BUTTON_DANGER = "danger";
export type ButtonVariant =
  typeof BUTTON_DEFAULT | typeof BUTTON_PRIMARY | typeof BUTTON_DANGER;

// Badge semantic status. `neutral` is the plain bordered pill; the rest fill
// with the matching semantic token color.
export const BADGE_OK = "ok";
export const BADGE_WARN = "warn";
export const BADGE_CRIT = "crit";
export const BADGE_INFO = "info";
export const BADGE_NEUTRAL = "neutral";
export type BadgeVariant =
  | typeof BADGE_OK
  | typeof BADGE_WARN
  | typeof BADGE_CRIT
  | typeof BADGE_INFO
  | typeof BADGE_NEUTRAL;

// StateBlock variants: loading + empty share the muted dashed scaffold look;
// error switches to a solid critical border with the raw message shown mono.
export const STATE_LOADING = "loading";
export const STATE_EMPTY = "empty";
export const STATE_ERROR = "error";
export type StateVariant =
  typeof STATE_LOADING | typeof STATE_EMPTY | typeof STATE_ERROR;
