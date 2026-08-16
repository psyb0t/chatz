<script lang="ts">
  import type { BaseComponentProps } from "@json-render/svelte";
  import { COMP_HEADING, DATA_JR_TYPE } from "$lib/common/render";

  type HeadingProps = {
    id?: string | null;
    level?: number | null;
    content?: string | null;
  };

  const { props }: BaseComponentProps<HeadingProps> = $props();

  const level = $derived(
    props.level === 1 || props.level === 3 ? props.level : 2,
  );
</script>

<div
  class="jr-heading jr-heading--{level}"
  id={props.id ?? undefined}
  role="heading"
  aria-level={level}
  {...{ [DATA_JR_TYPE]: COMP_HEADING }}
>
  {props.content ?? ""}
</div>

<style>
  .jr-heading {
    min-width: 0;
    max-width: 100%;
    font-family: var(--font-display);
    font-weight: 600;
    letter-spacing: -0.01em;
    line-height: 1.2;
    color: var(--ink);
    overflow-wrap: anywhere;
  }

  .jr-heading--1 {
    font-size: var(--text-2xl);
  }

  .jr-heading--2 {
    font-size: var(--text-xl);
  }

  .jr-heading--3 {
    font-size: var(--text-lg);
  }
</style>
