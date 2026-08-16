<script lang="ts">
  // Back-compat shim: StatusChip folded into the ui/Chip primitive. Kept so any
  // remaining importer still resolves; new code should import ui/Chip directly.
  import Chip from "$lib/components/ui/Chip.svelte";

  interface Props {
    label: string;
  }

  const { label }: Props = $props();
</script>

<Chip {label} />
