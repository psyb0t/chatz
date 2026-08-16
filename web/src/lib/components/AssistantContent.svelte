<script lang="ts">
  // Renders an assistant message body as an ORDERED list of segments: prose runs
  // render as streaming plain text (unchanged from the old behavior), ```spec
  // blocks render as live json-render component trees inline where the block sat.
  //
  // Streaming: the conversation store accumulates the full message text as deltas
  // arrive, so we re-derive the segments from the complete text on every change.
  // This is equivalent to feeding a FenceParser incrementally (json-render
  // tolerates a half-built spec — missing children are skipped) but keeps the
  // render layer a pure function of message.text. The raw JSONL of a ```spec
  // block is NEVER shown as text; a spec that is still open at stream end keeps
  // rendering whatever patches already parsed.
  import { parseMessageSegments, SEGMENT_SPEC } from "$lib/render/fence";
  import Renderer from "$lib/render/Renderer.svelte";
  import Markdown from "$lib/components/markdown/Markdown.svelte";
  import { log } from "$lib/log";
  import {
    EVENT_SPEC_OPEN,
    EVENT_SPEC_PATCH,
    EVENT_SPEC_RENDER,
    EVENT_SPEC_ERROR,
  } from "$lib/common/log-events";
  import {
    TESTID_ASSISTANT_TEXT,
    TESTID_SPEC_BLOCK,
  } from "$lib/common/test-ids";

  interface Props {
    text: string;
    streaming: boolean;
  }

  const { text, streaming }: Props = $props();

  // Re-segment on every text change. FenceParser is stateless-per-call by
  // design (see its REUSE TRIPWIRE doc comment), so parseMessageSegments
  // replays the FULL accumulated text from scratch on every delta — which
  // means every structural callback (onSpecOpen/onPatch/onSpecError) refires
  // for content that was ALREADY reported on a prior delta, not just the new
  // increment. Left unguarded, a message with one open spec block logs
  // spec.open (and spec.patch, and any spec.error) again on every single SSE
  // delta for the rest of the stream — pure noise, and on a large/slow
  // message it's a lot of it. The watermarks below are plain (non-reactive)
  // component-scoped counters that persist across re-derives for this block
  // (the block's id is stable — only its text grows) and gate each callback
  // to fire once per NEWLY-reached count, not once per replay.
  let specOpenWatermark = 0;
  let errorWatermark = 0;
  const patchWatermarks: number[] = [];

  const segments = $derived.by(() => {
    let specOpenSeen = 0;
    let errorSeen = 0;

    // finalize=!streaming: while this turn is still streaming, a patch that
    // looks incomplete on THIS delta may simply not have arrived in full yet
    // — only report it as a genuine "unterminated spec line" once streaming
    // has actually finished, otherwise a multi-chunk patch (e.g. a large
    // table) fires that error on every intermediate delta instead of never.
    return parseMessageSegments(
      text,
      {
        onSpecOpen: () => {
          specOpenSeen += 1;
          if (specOpenSeen > specOpenWatermark) {
            specOpenWatermark = specOpenSeen;
            log.info(EVENT_SPEC_OPEN, {});
          }
        },
        onPatch: (index, count) => {
          if (count > (patchWatermarks[index] ?? 0)) {
            patchWatermarks[index] = count;
            log.debug(EVENT_SPEC_PATCH, { segment: index, count });
          }
        },
        onSpecError: (message) => {
          errorSeen += 1;
          if (errorSeen > errorWatermark) {
            errorWatermark = errorSeen;
            log.error(EVENT_SPEC_ERROR, { message });
          }
        },
      },
      !streaming,
    );
  });

  // Log a render summary once per spec segment per DISTINCT (elements,
  // closed) shape — same replay-dedup reasoning as above, otherwise this
  // logs on every delta even when nothing about the segment changed.
  const renderLogged: string[] = [];
  $effect(() => {
    for (const [i, seg] of segments.entries()) {
      if (seg.kind !== SEGMENT_SPEC) {
        continue;
      }

      const elements = Object.keys(seg.spec.elements).length;
      const shape = `${elements}:${seg.closed}`;
      if (renderLogged[i] === shape) {
        continue;
      }

      renderLogged[i] = shape;
      log.debug(EVENT_SPEC_RENDER, { elements, closed: seg.closed });
    }
  });
</script>

{#each segments as segment, i (i)}
  {#if segment.kind === SEGMENT_SPEC}
    <div class="assistant__spec" data-testid={TESTID_SPEC_BLOCK}>
      <Renderer spec={segment.spec} loading={streaming && !segment.closed} />
    </div>
  {:else}
    <div
      class="assistant__text message__text--stream"
      data-testid={TESTID_ASSISTANT_TEXT}
    >
      <Markdown
        md={segment.text}
      />{#if streaming && i === segments.length - 1}<span
          class="assistant__caret"
          aria-hidden="true">█</span
        >{/if}
    </div>
  {/if}
{/each}

<!-- Caret placeholder for a streaming turn whose text hasn't arrived yet, or
     whose last segment is a spec block (no trailing prose to carry the caret). -->
{#if streaming && (segments.length === 0 || segments[segments.length - 1]?.kind === SEGMENT_SPEC)}
  <p class="assistant__text" data-testid={TESTID_ASSISTANT_TEXT}>
    <span class="assistant__caret" aria-hidden="true">█</span>
  </p>
{/if}

<style>
  .assistant__spec {
    min-width: 0;
    max-width: 100%;
    margin: var(--space-2) 0;
    overflow-x: clip;
  }

  .assistant__spec :global([data-jr-type]) {
    transform-origin: center top;
    animation: genui-component-reveal 240ms cubic-bezier(0.2, 0.8, 0.2, 1) both;
  }

  .assistant__text {
    margin: 0;
    word-break: break-word;
  }

  .assistant__caret {
    color: var(--accent);
    animation: blink 1s steps(1) infinite;
  }

  @keyframes blink {
    50% {
      opacity: 0;
    }
  }

  @keyframes genui-component-reveal {
    from {
      opacity: 0;
      transform: scale(0.94);
    }

    to {
      opacity: 1;
      transform: scale(1);
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .assistant__spec :global([data-jr-type]),
    .assistant__caret {
      animation: none;
    }
  }
</style>
