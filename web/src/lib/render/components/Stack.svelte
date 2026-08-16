<script lang="ts">
  import type { Snippet } from "svelte";
  import type { BaseComponentProps } from "@json-render/svelte";
  import { COMP_STACK, DATA_JR_TYPE } from "$lib/common/render";

  type StackProps = {
    id?: string | null;
    direction?: "vertical" | "horizontal" | null;
    gap?: "sm" | "md" | "lg" | null;
  };

  interface Props extends BaseComponentProps<StackProps> {
    children?: Snippet;
  }

  const { props, children }: Props = $props();

  const direction = $derived(
    props.direction === "horizontal" ? "horizontal" : "vertical",
  );
  const gap = $derived(
    props.gap === "sm" || props.gap === "lg" ? props.gap : "md",
  );
</script>

<div
  class="jr-stack jr-stack--{direction} jr-stack--gap-{gap}"
  id={props.id ?? undefined}
  {...{ [DATA_JR_TYPE]: COMP_STACK }}
>
  {#if children}
    {@render children()}
  {/if}
</div>

<style>
  .jr-stack {
    display: flex;
    min-width: 0;
    max-width: 100%;
  }

  .jr-stack > :global(*) {
    min-width: 0;
    max-width: 100%;
  }

  .jr-stack--vertical {
    flex-direction: column;
  }

  .jr-stack--horizontal {
    flex-direction: row;
    flex-wrap: wrap;
    align-items: flex-start;
  }

  .jr-stack--gap-sm {
    gap: var(--space-2);
  }

  .jr-stack--gap-md {
    gap: var(--space-4);
  }

  .jr-stack--gap-lg {
    gap: var(--space-6);
  }
</style>
