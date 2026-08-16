<script lang="ts">
  interface Props {
    label: string;
    id?: string;
    testid?: string;
  }

  const { label, id, testid }: Props = $props();
</script>

<span class="chip" {id} data-testid={testid}>{label}</span>

<style>
  .chip {
    display: inline-block;
    border: var(--border-width) solid var(--border);
    border-radius: 999px;
    padding: var(--space-1) var(--space-3);
    font-family: var(--font-display);
    font-size: var(--text-xs);
    font-weight: 500;
    color: var(--muted);
    background: var(--panel);
    white-space: nowrap;
  }
</style>
