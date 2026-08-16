<script lang="ts">
  import type { BaseComponentProps } from "@json-render/svelte";
  import { COMP_BADGE, DATA_JR_TYPE } from "$lib/common/render";
  import Badge from "$lib/components/ui/Badge.svelte";
  import { BADGE_INFO, type BadgeVariant } from "$lib/components/ui/variants";

  type BadgeProps = {
    id?: string | null;
    label?: string | null;
    variant?: "ok" | "warn" | "crit" | "info" | null;
  };

  const { props }: BaseComponentProps<BadgeProps> = $props();

  const variant = $derived<BadgeVariant>(
    props.variant === "ok" ||
      props.variant === "warn" ||
      props.variant === "crit"
      ? props.variant
      : BADGE_INFO,
  );
</script>

<Badge
  label={props.label ?? ""}
  {variant}
  id={props.id ?? undefined}
  rootAttrs={{ [DATA_JR_TYPE]: COMP_BADGE }}
/>
