// Structured client logger. Emits ONE parseable line per event to console.* so
// the browser e2e harness can capture + grep it: a stable prefix followed by a
// single JSON string arg (payload). Keeping the payload in one string arg means
// the captured console `text` is clean and greppable rather than split across
// multiple console arguments.
//
// This is the ONLY sanctioned place in the app where console.* is used — every
// other module logs through this API and stays console-free.
//
// NEVER pass secrets (passwords, cookies, tokens, raw request bodies) as fields.
// Callers are responsible for redaction/omission; there is no request-body path
// through here by design.
import {
  LEVEL_DEBUG,
  LEVEL_ERROR,
  LEVEL_INFO,
  LEVEL_WARN,
  LOG_LEVEL_QUERY_PARAM,
  LOG_LEVEL_STORAGE_KEY,
  LOG_PREFIX,
  type LogLevel,
} from "$lib/common/log";

type LogFields = Record<string, string | number | boolean | null | undefined>;

interface LogPayload extends LogFields {
  level: LogLevel;
  event: string;
  seq: number;
}

const LEVEL_ORDER: Record<LogLevel, number> = {
  [LEVEL_DEBUG]: 10,
  [LEVEL_INFO]: 20,
  [LEVEL_WARN]: 30,
  [LEVEL_ERROR]: 40,
};

function readQueryLevel(): LogLevel | null {
  if (typeof window === "undefined") {
    return null;
  }

  const raw = new URLSearchParams(window.location.search).get(
    LOG_LEVEL_QUERY_PARAM,
  );

  return isLevel(raw) ? raw : null;
}

function readStorageLevel(): LogLevel | null {
  if (typeof localStorage === "undefined") {
    return null;
  }

  try {
    const raw = localStorage.getItem(LOG_LEVEL_STORAGE_KEY);

    return isLevel(raw) ? raw : null;
  } catch {
    return null;
  }
}

function isLevel(value: string | null): value is LogLevel {
  return (
    value === LEVEL_DEBUG ||
    value === LEVEL_INFO ||
    value === LEVEL_WARN ||
    value === LEVEL_ERROR
  );
}

// Threshold gate: query param wins, then localStorage, then the environment
// default (debug in Vite dev, info in a production build).
function resolveThreshold(): LogLevel {
  const fromQuery = readQueryLevel();
  if (fromQuery) {
    return fromQuery;
  }

  const fromStorage = readStorageLevel();
  if (fromStorage) {
    return fromStorage;
  }

  return import.meta.env.DEV ? LEVEL_DEBUG : LEVEL_INFO;
}

const CONSOLE_METHOD: Record<LogLevel, (message: string) => void> = {
  [LEVEL_DEBUG]: (message) => console.debug(message),
  [LEVEL_INFO]: (message) => console.info(message),
  [LEVEL_WARN]: (message) => console.warn(message),
  [LEVEL_ERROR]: (message) => console.error(message),
};

class Logger {
  private seq = 0;
  private threshold: LogLevel = LEVEL_INFO;

  constructor() {
    this.threshold = resolveThreshold();
  }

  debug(event: string, fields?: LogFields): void {
    this.emit(LEVEL_DEBUG, event, fields);
  }

  info(event: string, fields?: LogFields): void {
    this.emit(LEVEL_INFO, event, fields);
  }

  warn(event: string, fields?: LogFields): void {
    this.emit(LEVEL_WARN, event, fields);
  }

  error(event: string, fields?: LogFields): void {
    this.emit(LEVEL_ERROR, event, fields);
  }

  private emit(level: LogLevel, event: string, fields?: LogFields): void {
    if (LEVEL_ORDER[level] < LEVEL_ORDER[this.threshold]) {
      return;
    }

    this.seq += 1;

    const payload: LogPayload = {
      level,
      event,
      seq: this.seq,
      ...fields,
    };

    CONSOLE_METHOD[level](`${LOG_PREFIX} ${JSON.stringify(payload)}`);
  }
}

export const log = new Logger();
