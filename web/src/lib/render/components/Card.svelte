<script lang="ts">
  import type { Snippet } from "svelte";
  import type { BaseComponentProps } from "@json-render/svelte";
  import { COMP_CARD, DATA_JR_TYPE } from "$lib/common/render";
  import Card from "$lib/components/ui/Card.svelte";

  type CardProps = {
    id?: string | null;
    title?: string | null;
    description?: string | null;
  };

  interface Props extends BaseComponentProps<CardProps> {
    children?: Snippet;
  }

  const { props, children }: Props = $props();
</script>

<Card
  title={props.title}
  description={props.description}
  id={props.id ?? undefined}
  rootAttrs={{ [DATA_JR_TYPE]: COMP_CARD }}
>
  {#if children}
    {@render children()}
  {/if}
</Card>
