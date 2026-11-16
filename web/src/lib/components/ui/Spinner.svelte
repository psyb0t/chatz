<script lang="ts">
  interface Props {
    size?: number;
  }

  const { size = 14 }: Props = $props();
</script>

<span
  class="spinner"
  style:width="{size}px"
  style:height="{size}px"
  aria-hidden="true"
></span>

<style>
  .spinner {
    display: inline-block;
    flex-shrink: 0;
    border: 2px solid var(--border-strong);
    border-top-color: var(--ink);
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }
</style>
