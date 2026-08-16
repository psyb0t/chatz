<script lang="ts">
  import type { BaseComponentProps } from "@json-render/svelte";
  import { COMP_TIMELINE, DATA_JR_TYPE } from "$lib/common/render";

  interface TimelineItem {
    time: string;
    label: string;
    detail?: string | null;
  }

  type TimelineProps = {
    id?: string | null;
    items?: TimelineItem[] | null;
  };

  const { props }: BaseComponentProps<TimelineProps> = $props();

  const items = $derived(Array.isArray(props.items) ? props.items : []);
</script>

<ol
  class="jr-timeline"
  id={props.id ?? undefined}
  {...{ [DATA_JR_TYPE]: COMP_TIMELINE }}
>
  {#each items as item, i (i)}
    <li class="jr-timeline__item">
      <div class="jr-timeline__time">{item.time}</div>
      <div class="jr-timeline__body">
        <div class="jr-timeline__label">{item.label}</div>
        {#if item.detail}
          <div class="jr-timeline__detail">{item.detail}</div>
        {/if}
      </div>
    </li>
  {/each}
</ol>

<style>
  .jr-timeline {
    min-width: 0;
    max-width: 100%;
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
  }

  .jr-timeline__item {
    display: flex;
    min-width: 0;
    gap: var(--space-3);
    padding: var(--space-2) 0;
    border-left: var(--border-width) solid var(--border);
    padding-left: var(--space-3);
    margin-left: var(--space-2);
  }

  .jr-timeline__time {
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    font-weight: 500;
    color: var(--accent);
    flex: 0 1 3.5rem;
    min-width: 0;
    overflow-wrap: anywhere;
  }

  .jr-timeline__body {
    display: flex;
    flex-direction: column;
    min-width: 0;
    gap: var(--space-1);
  }

  .jr-timeline__label {
    overflow-wrap: anywhere;
    font-family: var(--font-display);
    font-weight: 600;
    font-size: var(--text-sm);
  }

  .jr-timeline__detail {
    overflow-wrap: anywhere;
    font-size: var(--text-sm);
    color: var(--muted);
  }
</style>
