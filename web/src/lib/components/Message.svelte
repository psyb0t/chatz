<script lang="ts">
  import type { ConvMessage } from "#lib/stores/conversation.svelte.js";
  import {
    ROLE_ASSISTANT,
    BLOCK_KIND_TEXT,
    BLOCK_KIND_THINKING,
    BLOCK_KIND_TOOL,
    messageText,
  } from "#lib/stores/conversation.svelte.js";
  import ToolCard from "#lib/components/ToolCard.svelte";
  import ThinkingBlock from "#lib/components/ThinkingBlock.svelte";
  import AssistantContent from "#lib/components/AssistantContent.svelte";
  import Icon from "#lib/components/ui/Icon.svelte";
  import { TESTID_MESSAGE } from "#lib/common/test-ids.js";
  import { A11Y_COPY, LABEL_COPIED } from "#lib/common/labels.js";

  interface Props {
    message: ConvMessage;
  }

  const { message }: Props = $props();

  const isAssistant = $derived(message.role === ROLE_ASSISTANT);
  // The live caret rides only the LAST block of a streaming turn — a mid-stream
  // text run followed by a tool call must not keep blinking.
  const lastIndex = $derived(message.blocks.length - 1);

  const COPIED_RESET_MS = 1500;
  let copied = $state(false);

  const replyText = $derived(
    message.blocks
      .filter((block) => block.kind === BLOCK_KIND_TEXT)
      .map((block) => (block.kind === BLOCK_KIND_TEXT ? block.text : ""))
      .join("\n\n")
      .trim(),
  );

  async function copyReply(): Promise<void> {
    try {
      await navigator.clipboard.writeText(replyText);
      copied = true;
      setTimeout(() => (copied = false), COPIED_RESET_MS);
    } catch {
      // Clipboard access can be refused (insecure origin, permissions); the
      // button simply does nothing then.
    }
  }
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
        <span class="message__pending" aria-hidden="true">
          <span></span><span></span><span></span>
        </span>
      {/if}

      {#if !message.streaming && replyText !== ""}
        <div class="message__actions">
          <button
            class="message__action"
            type="button"
            onclick={copyReply}
            aria-label={A11Y_COPY}
            title={copied ? LABEL_COPIED : A11Y_COPY}
          >
            <Icon name={copied ? "check" : "copy"} size={14} />
          </button>
        </div>
      {/if}
    </div>
  {:else}
    <!-- User: a right-aligned bubble, capped at 80% of the column. -->
    <div class="message__bubble">{messageText(message)}</div>
  {/if}
</div>

<style>
  .message {
    min-width: 0;
    max-width: 100%;
    animation: fade-in var(--dur) var(--ease);
  }

  .message--user {
    display: flex;
    justify-content: flex-end;
  }

  .message__bubble {
    max-width: 80%;
    border-radius: var(--radius-xl);
    background: var(--bubble);
    color: var(--ink);
    padding: var(--space-2) var(--space-4);
    font-size: var(--text-prose);
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
    gap: var(--space-3);
    font-size: var(--text-prose);
  }

  .message__pending {
    display: inline-flex;
    gap: 4px;
    padding: var(--space-2) 0;
  }

  .message__pending span {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--faint);
    animation: pulse 1.2s ease-in-out infinite;
  }

  .message__pending span:nth-child(2) {
    animation-delay: 0.15s;
  }

  .message__pending span:nth-child(3) {
    animation-delay: 0.3s;
  }

  .message__actions {
    display: flex;
    gap: var(--space-1);
    margin-top: calc(-1 * var(--space-1));
    opacity: 0;
    transition: opacity var(--dur-fast) var(--ease);
  }

  .message:hover .message__actions,
  .message__actions:focus-within {
    opacity: 1;
  }

  .message__action {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: var(--control-sm);
    height: var(--control-sm);
    padding: 0;
    border: none;
    border-radius: var(--radius-sm);
    background: transparent;
    color: var(--faint);
  }

  .message__action:hover:not(:disabled) {
    background: var(--fill-hover);
    color: var(--ink);
  }

  .message__interrupted {
    margin: 0;
    color: var(--muted);
    font-size: var(--text-sm);
  }

  @keyframes pulse {
    0%,
    80%,
    100% {
      opacity: 0.25;
      transform: scale(0.85);
    }

    40% {
      opacity: 1;
      transform: none;
    }
  }

  @media (hover: none) {
    .message__actions {
      opacity: 1;
    }
  }
</style>
