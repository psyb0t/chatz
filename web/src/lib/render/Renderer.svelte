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

  interface Props {
    spec: Spec;
    loading?: boolean;
  }

  const { spec, loading = false }: Props = $props();

  // Stamp element-key ids onto any element whose props.id is null before render
  // (frontend allocates ids — chatz has no server-side finalize). Re-runs
  // reactively as the spec grows during streaming.
  const stamped = $derived(stampSpecIds(spec));

  // json-render resolves the tree from /root and renders nothing at all when
  // that key names an element the model never defined — indistinguishable from
  // "the assistant chose not to draw". A model that set /root to "main" while
  // naming its element "cardMain" therefore produced a permanently blank block,
  // on first render and on every reload of the persisted message.
  //
  // Only checked once the block is complete: mid-stream /root legitimately
  // arrives before the element it points at, and rule 19 has leaves emitted
  // before their parents, so a partial tree is expected to be unresolvable.
  const issues = $derived(loading ? [] : validateSpec(stamped).issues);
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
    <JsonUIProvider initialState={stamped.state}>
      <Renderer spec={stamped} {registry} {loading} />
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
