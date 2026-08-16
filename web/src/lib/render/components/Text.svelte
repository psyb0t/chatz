<script lang="ts">
  import type { BaseComponentProps } from "@json-render/svelte";
  import { COMP_TEXT, DATA_JR_TYPE } from "$lib/common/render";

  type TextProps = {
    id?: string | null;
    content?: string | null;
  };

  const { props }: BaseComponentProps<TextProps> = $props();
</script>

<p
  class="jr-text"
  id={props.id ?? undefined}
  {...{ [DATA_JR_TYPE]: COMP_TEXT }}
>
  {props.content ?? ""}
</p>

<style>
  .jr-text {
    min-width: 0;
    max-width: 100%;
    margin: 0;
    font-family: var(--font-display);
    font-size: var(--text-base);
    line-height: 1.5;
    color: var(--ink);
    overflow-wrap: anywhere;
  }
</style>
