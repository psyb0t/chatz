// Constants for the client logger: the console line prefix, the storage key +
// query param that raise the level to debug, and the level names. Kept out of
// log.ts so the values are importable by tests without pulling in console I/O.
export const LOG_PREFIX = "CHATZ_LOG";
export const LOG_LEVEL_STORAGE_KEY = "chatz_log_level";
export const LOG_LEVEL_QUERY_PARAM = "log";

export const LEVEL_DEBUG = "debug";
export const LEVEL_INFO = "info";
export const LEVEL_WARN = "warn";
export const LEVEL_ERROR = "error";

export type LogLevel =
  | typeof LEVEL_DEBUG
  | typeof LEVEL_INFO
  | typeof LEVEL_WARN
  | typeof LEVEL_ERROR;
