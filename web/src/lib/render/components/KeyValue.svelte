<script lang="ts">
  import type { BaseComponentProps } from "@json-render/svelte";
  import { COMP_KEY_VALUE, DATA_JR_TYPE } from "$lib/common/render";

  interface KeyValueItem {
    label: string;
    value: string;
  }

  type KeyValueProps = {
    id?: string | null;
    items?: KeyValueItem[] | null;
  };

  const { props }: BaseComponentProps<KeyValueProps> = $props();

  const items = $derived(Array.isArray(props.items) ? props.items : []);
</script>

<dl
  class="jr-kv"
  id={props.id ?? undefined}
  {...{ [DATA_JR_TYPE]: COMP_KEY_VALUE }}
>
  {#each items as item, i (i)}
    <div class="jr-kv__row">
      <dt class="jr-kv__label">{item.label}</dt>
      <dd class="jr-kv__value">{item.value}</dd>
    </div>
  {/each}
</dl>

<style>
  .jr-kv {
    min-width: 0;
    max-width: 100%;
    margin: 0;
    border: var(--border-width) solid var(--border);
    border-radius: var(--radius);
    overflow: hidden;
    background: var(--panel);
  }

  .jr-kv__row {
    display: flex;
    min-width: 0;
    justify-content: space-between;
    gap: var(--space-4);
    padding: var(--space-2) var(--space-3);
    border-bottom: var(--border-width) solid var(--border);
  }

  .jr-kv__row:last-child {
    border-bottom: none;
  }

  .jr-kv__label {
    min-width: 0;
    overflow-wrap: anywhere;
    font-family: var(--font-display);
    font-size: var(--text-sm);
    font-weight: 500;
    color: var(--muted);
  }

  .jr-kv__value {
    min-width: 0;
    margin: 0;
    font-family: var(--font-mono);
    font-size: var(--text-sm);
    text-align: right;
    overflow-wrap: anywhere;
  }
</style>
