<script lang="ts">
  import type { BaseComponentProps } from "@json-render/svelte";
  import { COMP_LOG_VIEWER, DATA_JR_TYPE } from "$lib/common/render";
  import type { LogEntry, LogLevel } from "$lib/render/charts/types";

  type LogViewerProps = {
    id?: string | null;
    title?: string | null;
    entries?: LogEntry[] | null;
    wrap?: boolean | null;
    maxHeight?: number | null;
  };

  const DEFAULT_MAX_HEIGHT = 360;
  const MIN_MAX_HEIGHT = 160;
  const MAX_MAX_HEIGHT = 800;
  const MAX_ENTRIES = 2_000;
  const VALID_LEVELS = new Set<LogLevel>(["debug", "info", "warn", "error"]);

  const { props }: BaseComponentProps<LogViewerProps> = $props();

  function safeText(value: unknown): string {
    return typeof value === "string" ? value : "";
  }

  function normalizeEntries(value: unknown): LogEntry[] {
    if (!Array.isArray(value)) {
      return [];
    }

    return value.slice(0, MAX_ENTRIES).flatMap((candidate) => {
      if (typeof candidate !== "object" || candidate === null) {
        return [];
      }

      const record = candidate as Record<string, unknown>;
      const level = safeText(record.level) as LogLevel;
      const message = safeText(record.message);
      if (!VALID_LEVELS.has(level) || !message) {
        return [];
      }

      return [
        {
          time: safeText(record.time),
          level,
          source: safeText(record.source) || null,
          message,
        },
      ];
    });
  }

  function safeMaxHeight(value: unknown): number {
    if (typeof value !== "number" || !Number.isFinite(value)) {
      return DEFAULT_MAX_HEIGHT;
    }

    return Math.min(
      Math.max(Math.round(value), MIN_MAX_HEIGHT),
      MAX_MAX_HEIGHT,
    );
  }

  const rootID = $derived(typeof props.id === "string" ? props.id : undefined);
  const title = $derived(safeText(props.title));
  const entries = $derived(normalizeEntries(props.entries));
  const wrap = $derived(props.wrap === true);
  const maxHeight = $derived(safeMaxHeight(props.maxHeight));
  const summary = $derived.by(() => {
    if (entries.length === 0) {
      return "No valid log entries are available.";
    }

    const counts = entries.reduce<Record<LogLevel, number>>(
      (totals, entry) => {
        totals[entry.level] += 1;
        return totals;
      },
      { debug: 0, info: 0, warn: 0, error: 0 },
    );
    const noun = entries.length === 1 ? "entry" : "entries";
    return `${entries.length} log ${noun}: ${counts.error} error, ${counts.warn} warning, ${counts.info} info, and ${counts.debug} debug.`;
  });
</script>

<section
  class="jr-logs"
  id={rootID}
  aria-label={title || "Log viewer"}
  {...{ [DATA_JR_TYPE]: COMP_LOG_VIEWER }}
>
  {#if title}
    <h3>{title}</h3>
  {/if}
  <p class="jr-logs__sr">{summary}</p>

  {#if entries.length === 0}
    <div class="jr-logs__empty" role="status">No valid log entries</div>
  {:else}
    <div
      class="jr-logs__viewport"
      class:jr-logs__viewport--wrap={wrap}
      style:max-height={`${maxHeight}px`}
      role="log"
      aria-live="off"
      aria-label={title || "Log entries"}
    >
      <ol>
        {#each entries as entry, index (index)}
          <li class="jr-logs__entry jr-logs__entry--{entry.level}">
            <span class="jr-logs__time">{entry.time}</span>
            <span class="jr-logs__level">{entry.level.toUpperCase()}</span>
            <span class="jr-logs__source">{entry.source ?? ""}</span>
            <span class="jr-logs__message">{entry.message}</span>
          </li>
        {/each}
      </ol>
    </div>
  {/if}
</section>

<style>
  .jr-logs {
    min-width: 0;
    border: var(--border-width) solid var(--border);
    border-radius: var(--radius);
    background: var(--panel);
    overflow: hidden;
  }

  h3 {
    margin: 0;
    padding: var(--space-3) var(--space-4);
    border-bottom: var(--border-width) solid var(--border);
    overflow-wrap: anywhere;
    font-family: var(--font-display);
    font-size: var(--text-base);
  }

  .jr-logs__viewport {
    overflow: auto;
    overscroll-behavior: contain;
    background: var(--panel-2);
  }

  ol {
    list-style: none;
    margin: 0;
    padding: 0;
    width: max-content;
    min-width: 100%;
  }

  .jr-logs__viewport--wrap ol {
    width: 100%;
  }

  .jr-logs__entry {
    display: grid;
    grid-template-columns: minmax(8rem, auto) 4.5rem minmax(6rem, auto) minmax(
        16rem,
        1fr
      );
    gap: var(--space-2);
    padding: var(--space-1) var(--space-3);
    border-bottom: var(--border-width) solid var(--border);
    border-left: 3px solid transparent;
    color: var(--ink);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    line-height: 1.5;
  }

  .jr-logs__entry:last-child {
    border-bottom: 0;
  }

  .jr-logs__entry--debug {
    border-left-color: var(--faint);
  }

  .jr-logs__entry--info {
    border-left-color: var(--accent);
  }

  .jr-logs__entry--warn {
    border-left-color: var(--warn);
  }

  .jr-logs__entry--error {
    border-left-color: var(--crit);
  }

  .jr-logs__time,
  .jr-logs__source {
    color: var(--muted);
  }

  .jr-logs__level {
    font-weight: 700;
  }

  .jr-logs__entry--debug .jr-logs__level {
    color: var(--faint);
  }

  .jr-logs__entry--info .jr-logs__level {
    color: var(--accent);
  }

  .jr-logs__entry--warn .jr-logs__level {
    color: var(--warn);
  }

  .jr-logs__entry--error .jr-logs__level {
    color: var(--crit);
  }

  .jr-logs__viewport:not(.jr-logs__viewport--wrap) .jr-logs__time,
  .jr-logs__viewport:not(.jr-logs__viewport--wrap) .jr-logs__source,
  .jr-logs__viewport:not(.jr-logs__viewport--wrap) .jr-logs__message {
    white-space: pre;
  }

  .jr-logs__viewport--wrap .jr-logs__entry {
    grid-template-columns: minmax(6rem, 9rem) 4.5rem minmax(5rem, 9rem) minmax(
        0,
        1fr
      );
  }

  .jr-logs__viewport--wrap .jr-logs__time,
  .jr-logs__viewport--wrap .jr-logs__source,
  .jr-logs__viewport--wrap .jr-logs__message {
    min-width: 0;
    overflow-wrap: anywhere;
    white-space: pre-wrap;
  }

  .jr-logs__empty {
    min-height: 8rem;
    display: grid;
    place-items: center;
    padding: var(--space-4);
    color: var(--muted);
    font-family: var(--font-mono);
    font-size: var(--text-sm);
  }

  .jr-logs__sr {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    margin: -1px;
    overflow: hidden;
    clip: rect(0, 0, 0, 0);
    white-space: nowrap;
    border: 0;
  }

  @media (max-width: 40rem) {
    .jr-logs__viewport--wrap .jr-logs__entry {
      grid-template-columns: minmax(5rem, 1fr) 4.5rem;
    }

    .jr-logs__viewport--wrap .jr-logs__source,
    .jr-logs__viewport--wrap .jr-logs__message {
      grid-column: 1 / -1;
    }
  }
</style>
