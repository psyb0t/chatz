<script lang="ts">
  import { BADGE_NEUTRAL, type BadgeVariant } from "./variants";

  interface Props {
    label: string;
    variant?: BadgeVariant;
    id?: string;
    testid?: string;
    // Extra root attributes (e.g. the json-render data-jr-type stamp). Forwarded
    // verbatim onto the outermost element so callers can attach contract attrs.
    rootAttrs?: Record<string, string>;
  }

  const {
    label,
    variant = BADGE_NEUTRAL,
    id,
    testid,
    rootAttrs = {},
  }: Props = $props();
</script>

<span class="badge badge--{variant}" {id} data-testid={testid} {...rootAttrs}>
  {label}
</span>

<style>
  .badge {
    display: inline-block;
    max-width: 100%;
    border-radius: 999px;
    padding: 0 var(--space-2);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    font-weight: 600;
    line-height: 1.8;
    color: var(--on-accent);
    overflow-wrap: anywhere;
  }

  .badge--neutral {
    background: var(--panel-2);
    color: var(--ink);
  }

  .badge--ok {
    background: var(--ok);
  }

  .badge--warn {
    background: var(--warn);
  }

  .badge--crit {
    background: var(--crit);
  }

  .badge--info {
    background: var(--accent);
  }
</style>
