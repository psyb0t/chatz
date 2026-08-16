<script lang="ts">
  // Renders a prose string as markdown via svelte-exmarkdown. The renderer walks
  // the remark/rehype AST and emits a real Svelte component tree — there is NO
  // {@html} in its render path and rehype-raw is NOT enabled, so raw HTML in the
  // model's output is escaped/dropped rather than executed. That makes it safe
  // for untrusted LLM text without a DOMPurify pass.
  //
  // Streaming-tolerant: `md` is reactive, so as the message text grows char by
  // char the tree re-renders; remark parses partial/unterminated markdown to a
  // best-effort tree without throwing.
  //
  // The `plugins` prop overrides a few tag renderers with token-styled brutalist
  // components (links, inline/block code, blockquote). gfmPlugin adds GitHub
  // tables/strikethrough/task-lists.
  import Markdown from "svelte-exmarkdown";
  import { gfmPlugin } from "svelte-exmarkdown/gfm";
  import type { Plugin } from "svelte-exmarkdown";
  import MdLink from "./MdLink.svelte";
  import MdCode from "./MdCode.svelte";
  import MdPre from "./MdPre.svelte";
  import MdBlockquote from "./MdBlockquote.svelte";

  interface Props {
    md: string;
  }

  const { md }: Props = $props();

  const plugins: Plugin[] = [
    gfmPlugin(),
    {
      renderer: {
        a: MdLink,
        code: MdCode,
        pre: MdPre,
        blockquote: MdBlockquote,
      },
    },
  ];
</script>

<div class="md">
  <Markdown {md} {plugins} />
</div>

<style>
  /* Tight, token-driven vertical rhythm for the generated element tree. Uses
     :global because svelte-exmarkdown emits the tags at runtime (scoped class
     hashing can't reach them). Scoped under .md so it never leaks to app chrome. */
  .md {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    word-break: break-word;
  }

  .md :global(> :first-child) {
    margin-top: 0;
  }

  .md :global(p),
  .md :global(ul),
  .md :global(ol),
  .md :global(pre),
  .md :global(blockquote),
  .md :global(table) {
    margin: 0;
  }

  .md :global(h1),
  .md :global(h2),
  .md :global(h3),
  .md :global(h4),
  .md :global(h5),
  .md :global(h6) {
    margin: var(--space-2) 0 0;
  }

  .md :global(ul),
  .md :global(ol) {
    padding-left: var(--space-6);
  }

  .md :global(table) {
    border-collapse: collapse;
    font-family: var(--font-mono);
    font-size: var(--text-sm);
    display: block;
    overflow-x: auto;
  }

  .md :global(th),
  .md :global(td) {
    border: var(--border-width) solid var(--border);
    padding: var(--space-1) var(--space-2);
    text-align: left;
  }

  .md :global(th) {
    background: var(--panel-2);
    color: var(--muted);
    font-weight: 600;
  }

  .md :global(hr) {
    border: 0;
    border-top: var(--border-width) solid var(--border);
    width: 100%;
  }
</style>
