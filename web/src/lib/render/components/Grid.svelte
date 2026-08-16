<script lang="ts">
  import type { Snippet } from "svelte";
  import type { BaseComponentProps } from "@json-render/svelte";
  import { COMP_GRID, DATA_JR_TYPE } from "$lib/common/render";

  type GridProps = {
    id?: string | null;
    columns?: number | null;
  };

  interface Props extends BaseComponentProps<GridProps> {
    children?: Snippet;
  }

  const { props, children }: Props = $props();

  // Clamp to a sane 1..6 range; missing/invalid falls back to 2 columns.
  const columns = $derived(
    typeof props.columns === "number" && props.columns >= 1
      ? Math.min(Math.floor(props.columns), 6)
      : 2,
  );
</script>

<div
  class="jr-grid"
  id={props.id ?? undefined}
  style="--jr-grid-cols: {columns}"
  {...{ [DATA_JR_TYPE]: COMP_GRID }}
>
  {#if children}
    {@render children()}
  {/if}
</div>

<style>
  .jr-grid {
    display: grid;
    min-width: 0;
    max-width: 100%;
    grid-template-columns: repeat(var(--jr-grid-cols, 2), minmax(0, 1fr));
    gap: var(--space-4);
  }

  .jr-grid > :global(*) {
    min-width: 0;
    max-width: 100%;
  }

  @media (max-width: 40rem) {
    .jr-grid {
      grid-template-columns: minmax(0, 1fr);
    }
  }
</style>
