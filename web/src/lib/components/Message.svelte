<script lang="ts">
  import type { ConvMessage } from "$lib/stores/conversation.svelte";
  import {
    ROLE_ASSISTANT,
    BLOCK_KIND_TEXT,
    BLOCK_KIND_THINKING,
    BLOCK_KIND_TOOL,
    messageText,
  } from "$lib/stores/conversation.svelte";
  import ToolCard from "$lib/components/ToolCard.svelte";
  import ThinkingBlock from "$lib/components/ThinkingBlock.svelte";
  import AssistantContent from "$lib/components/AssistantContent.svelte";
  import { TESTID_MESSAGE } from "$lib/common/test-ids";

  interface Props {
    message: ConvMessage;
  }

  const { message }: Props = $props();

  const isAssistant = $derived(message.role === ROLE_ASSISTANT);
  // The live caret rides only the LAST block of a streaming turn — a mid-stream
  // text run followed by a tool call must not keep blinking.
  const lastIndex = $derived(message.blocks.length - 1);
</script>

<div
  class="message message--{message.role}"
  data-testid={TESTID_MESSAGE}
  data-role={message.role}
>
  {#if isAssistant}
    <!-- Unbounded: the assistant writes straight onto the page, no wrapper box;
         each block (prose / thinking / tool card) carries its own chrome. -->
    <div class="message__assistant">
      {#if message.incomplete}
        <p class="message__interrupted">Interrupted response</p>
      {/if}
      {#each message.blocks as block, i (block.id)}
        {#if block.kind === BLOCK_KIND_TEXT}
          <AssistantContent
            text={block.text}
            streaming={message.streaming && i === lastIndex}
          />
        {:else if block.kind === BLOCK_KIND_THINKING}
          <ThinkingBlock
            text={block.text}
            streaming={message.streaming && i === lastIndex}
          />
        {:else if block.kind === BLOCK_KIND_TOOL}
          <ToolCard call={block.call} />
        {/if}
      {/each}

      {#if message.streaming && message.blocks.length === 0}
        <span class="message__caret" aria-hidden="true">█</span>
      {/if}
    </div>
  {:else}
    <!-- User: a right-aligned bubble, capped at 70% of the pane width. -->
    <div class="message__bubble">{messageText(message)}</div>
  {/if}
</div>

<style>
  .message {
    min-width: 0;
    max-width: 100%;
  }

  .message--user {
    display: flex;
    justify-content: flex-end;
  }

  .message__bubble {
    max-width: 70%;
    border: var(--border-width) solid var(--border);
    border-radius: var(--radius-lg);
    background: var(--accent-soft);
    color: var(--ink);
    padding: var(--space-3) var(--space-4);
    line-height: 1.6;
    white-space: pre-wrap;
    word-break: break-word;
  }

  /* Ordered blocks (text / thinking / tool) stack with consistent spacing so an
     interleaved turn reads as one flowing response. */
  .message__assistant {
    display: flex;
    flex-direction: column;
    min-width: 0;
    max-width: 100%;
    gap: var(--space-2);
  }

  .message__caret {
    color: var(--accent);
    animation: blink 1s steps(1) infinite;
  }

  .message__interrupted {
    margin: 0;
    color: var(--muted);
    font-size: var(--text-sm);
  }

  @keyframes blink {
    50% {
      opacity: 0;
    }
  }
</style>
