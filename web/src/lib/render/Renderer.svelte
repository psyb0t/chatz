<script lang="ts">
  import { Renderer, JsonUIProvider } from "@json-render/svelte";
  import type { Spec } from "@json-render/svelte";
  import { validateSpec } from "@json-render/core";
  import StateBlock from "$lib/components/ui/StateBlock.svelte";
  import { STATE_ERROR } from "$lib/components/ui/variants";
  import { SPEC_INVALID_LABEL, specIssueLine } from "$lib/common/labels";
  import { TESTID_SPEC_ERROR } from "$lib/common/test-ids";
  import { registry } from "./registry";
  import { stampSpecIds } from "./stamp";
  import { recoverRoot } from "./recover-root";

  interface Props {
    spec: Spec;
    loading?: boolean;
  }

  const { spec, loading = false }: Props = $props();

  // Stamp element-key ids onto any element whose props.id is null before render
  // (frontend allocates ids — chatz has no server-side finalize). Re-runs
  // reactively as the spec grows during streaming.
  const stamped = $derived(stampSpecIds(spec));

  // A model that set /root to "main" while keying its top element "stackMain"
  // leaves /root dangling, and json-render then draws nothing — indistinguishable
  // from "the assistant chose not to draw", on first render and every reload.
  // recoverRoot rewires /root to the real top element when it is unambiguous, so
  // an otherwise-complete tree renders instead of vanishing.
  //
  // Both recovery and validation are deferred until the block is complete:
  // mid-stream /root legitimately precedes the element it points at (rule 19
  // emits leaves before parents), so a partial tree is expected to be
  // unresolvable and must not be "recovered" against a transient shape.
  const rendered = $derived(loading ? stamped : recoverRoot(stamped));
  const issues = $derived(loading ? [] : validateSpec(rendered).issues);
  const errors = $derived(
    issues.filter((issue) => issue.severity === STATE_ERROR),
  );
</script>

<div class="jr-renderer">
  {#if errors.length > 0}
    <StateBlock
      variant={STATE_ERROR}
      label={SPEC_INVALID_LABEL}
      testid={TESTID_SPEC_ERROR}
    />
    <ul class="jr-renderer__issues">
      {#each errors as issue (issue.code + (issue.elementKey ?? ""))}
        <li>{specIssueLine(issue.code, issue.elementKey)}</li>
      {/each}
    </ul>
  {:else}
    <JsonUIProvider initialState={rendered.state}>
      <Renderer spec={rendered} {registry} {loading} />
    </JsonUIProvider>
  {/if}
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

  .jr-renderer__issues {
    margin: var(--space-2) 0 0;
    padding-left: var(--space-4);
    color: var(--muted);
    font-size: var(--text-xs);
    font-family: var(--font-mono);
  }
</style>
