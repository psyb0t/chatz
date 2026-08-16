// Auth phase names. The store and the layout guard switch on these.
export const PHASE_LOADING = "loading";
export const PHASE_SETUP = "setup";
export const PHASE_ANON = "anon";
export const PHASE_AUTHED = "authed";

export type AuthPhase =
  | typeof PHASE_LOADING
  | typeof PHASE_SETUP
  | typeof PHASE_ANON
  | typeof PHASE_AUTHED;
