// Theme store — light/dark, mirrored onto the <html data-theme> attribute and
// persisted in localStorage. The initial value is read back from whatever the
// app.html boot script already resolved onto documentElement, so the store
// never re-triggers a flash or disagrees with the pre-paint theme.

export const THEME_LIGHT = "light";
export const THEME_DARK = "dark";

export type Theme = typeof THEME_LIGHT | typeof THEME_DARK;

// Kept in sync with the storage key + attribute used by the app.html boot script.
const STORAGE_KEY = "chatz-theme";
const DATA_ATTR = "data-theme";

export const THEME_TOGGLE_LABEL = "Toggle theme";
export const TESTID_THEME_TOGGLE = "theme-toggle";

function readInitial(): Theme {
  if (typeof document === "undefined") {
    return THEME_LIGHT;
  }

  return document.documentElement.getAttribute(DATA_ATTR) === THEME_DARK
    ? THEME_DARK
    : THEME_LIGHT;
}

function persist(value: Theme): void {
  if (typeof document !== "undefined") {
    document.documentElement.setAttribute(DATA_ATTR, value);
  }

  try {
    localStorage.setItem(STORAGE_KEY, value);
  } catch {
    // Persistence unavailable (private mode / quota). Non-critical: the
    // attribute already applied for this session, only cross-reload memory is
    // lost. Mirrors the logger's own localStorage handling.
  }
}

class ThemeStore {
  current = $state<Theme>(readInitial());

  get isDark(): boolean {
    return this.current === THEME_DARK;
  }

  set(value: Theme): void {
    this.current = value;
    persist(value);
  }

  toggle(): void {
    this.set(this.current === THEME_DARK ? THEME_LIGHT : THEME_DARK);
  }
}

export const theme = new ThemeStore();
