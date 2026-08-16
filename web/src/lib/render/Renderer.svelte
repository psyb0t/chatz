<script lang="ts">
  import { Renderer, JsonUIProvider } from "@json-render/svelte";
  import type { Spec } from "@json-render/svelte";
  import { registry } from "./registry";
  import { stampSpecIds } from "./stamp";

  interface Props {
    spec: Spec;
    loading?: boolean;
  }

  const { spec, loading = false }: Props = $props();

  // Stamp element-key ids onto any element whose props.id is null before render
  // (frontend allocates ids — chatz has no server-side finalize). Re-runs
  // reactively as the spec grows during streaming.
  const stamped = $derived(stampSpecIds(spec));
</script>

<div class="jr-renderer">
  <JsonUIProvider initialState={stamped.state}>
    <Renderer spec={stamped} {registry} {loading} />
  </JsonUIProvider>
</div>

<style>
  .jr-renderer {
    min-width: 0;
    max-width: 100%;
  }

  .jr-renderer :global([data-jr-type]) {
    min-width: 0;
    max-width: 100%;
  }
</style>
