<script lang="ts">
  import { untrack } from "svelte";
  import Composer from "$lib/components/Composer.svelte";
  import Message from "$lib/components/Message.svelte";
  import Button from "$lib/components/ui/Button.svelte";
  import StateBlock from "$lib/components/ui/StateBlock.svelte";
  import {
    STATE_ERROR,
    STATE_LOADING as STATE_VARIANT_LOADING,
  } from "$lib/components/ui/variants";
  import { conversation } from "$lib/stores/conversation.svelte";
  import { chatTurnStatusLabel } from "$lib/common/turn-status";
  import {
    TESTID_MESSAGE_LIST,
    TESTID_CHAT_LOADING,
    TESTID_CHAT_ERROR,
    TESTID_CHAT_RETRY_TURN,
    TESTID_CHAT_TURN_STATUS,
  } from "$lib/common/test-ids";
  import {
    EMPTY_CONVERSATION,
    STATE_LOADING_HISTORY,
    LABEL_DISMISS,
    LABEL_RETRY,
    A11Y_CONVERSATION,
    turnElapsed,
  } from "$lib/common/labels";

  const messages = $derived(conversation.messages);
  const error = $derived(conversation.error);
  const loading = $derived(conversation.loading);
  const streaming = $derived(conversation.streaming);
  const turnStatus = $derived(conversation.turnStatus);
  const empty = $derived(messages.length === 0);
  // A load failure (no messages, not streaming) can be retried; a mid-stream
  // failure keeps the partial turn on screen, so only dismiss makes sense there.
  const canRetryLoad = $derived(!streaming && empty);
  const canRetryTurn = $derived(conversation.retryAvailable);
  const elapsedSeconds = $derived(conversation.elapsedSeconds);

  function dismiss(): void {
    conversation.dismissError();
  }

  function retry(): void {
    void conversation.retryLoad();
  }

  function retryTurn(): void {
    void conversation.retryLastTurn();
  }

  // Sticky auto-scroll: stay pinned to the newest content while streaming, but
  // detach the moment the user scrolls up — and resume when they return to the
  // bottom. Detach is driven off the input gesture (wheel/touch), NOT the scroll
  // event: a streamed delta can re-pin to the bottom in the frame between the
  // gesture and the scroll event firing, so reading intent from `scroll` alone
  // loses the scroll-up every time content is flowing.
  const BOTTOM_STICK_THRESHOLD_PX = 64;

  let listEl: HTMLDivElement | undefined = $state();
  let stuck = $state(true);
  // Plain (non-reactive) — compared across effect runs to detect a chat switch.
  let trackedChatId: string | null = null;
  let touchStartY = 0;

  function scrollToBottom(): void {
    if (listEl !== undefined) {
      listEl.scrollTop = listEl.scrollHeight;
    }
  }

  function distanceFromBottom(): number {
    if (listEl === undefined) {
      return 0;
    }

    return listEl.scrollHeight - listEl.scrollTop - listEl.clientHeight;
  }

  // A settled scroll reconciles the flag with position: at/near the bottom
  // re-attaches, scrolled up detaches. A programmatic scrollToBottom always
  // lands at distance 0, so it can only ever re-attach — it never fights a user
  // reading above.
  function onScroll(): void {
    stuck = distanceFromBottom() <= BOTTOM_STICK_THRESHOLD_PX;
  }

  // Upward wheel detaches immediately, synchronously with the gesture — before
  // a pending delta's auto-scroll can yank us back to the bottom.
  function onWheel(event: WheelEvent): void {
    if (event.deltaY < 0) {
      stuck = false;
    }
  }

  function onTouchStart(event: TouchEvent): void {
    touchStartY = event.touches[0]?.clientY ?? 0;
  }

  // Finger dragging down pulls content up (a scroll-up gesture) → detach.
  function onTouchMove(event: TouchEvent): void {
    const y = event.touches[0]?.clientY ?? 0;
    if (y > touchStartY) {
      stuck = false;
    }

    touchStartY = y;
  }

  // Switching chats (or starting a new one) snaps back to the newest message.
  $effect(() => {
    if (conversation.chatId !== trackedChatId) {
      trackedChatId = conversation.chatId;
      stuck = true;
    }
  });

  // The store replaces `messages` immutably on every streamed delta, so
  // reading the array itself here re-runs this effect as content arrives —
  // stay pinned to the bottom unless the user scrolled up. `stuck` is read via
  // untrack: onScroll flips it back to true the instant a scroll-up gesture
  // drifts back within BOTTOM_STICK_THRESHOLD_PX of the bottom (so re-sticking
  // after the user scrolls back down still works), but that flip must NOT by
  // itself re-run this effect — otherwise every small upward drag that
  // re-enters the threshold snaps the view back to the bottom mid-gesture,
  // fighting the user's own scroll instead of just gating future auto-scrolls.
  $effect(() => {
    if (messages.length > 0 && untrack(() => stuck)) {
      scrollToBottom();
    }
  });
</script>

<div class="chat" id="chat">
  <div
    class="chat__messages"
    data-testid={TESTID_MESSAGE_LIST}
    id="chat-messages"
    role="log"
    aria-label={A11Y_CONVERSATION}
    bind:this={listEl}
    onscroll={onScroll}
    onwheel={onWheel}
    ontouchstart={onTouchStart}
    ontouchmove={onTouchMove}
  >
    {#if loading}
      <StateBlock
        variant={STATE_VARIANT_LOADING}
        label={STATE_LOADING_HISTORY}
        testid={TESTID_CHAT_LOADING}
      />
    {:else if empty && error === null}
      <p class="chat__empty">{EMPTY_CONVERSATION}</p>
    {:else}
      {#each messages as message (message.id)}
        <Message {message} />
      {/each}
    {/if}

    {#if error !== null}
      <StateBlock
        variant={STATE_ERROR}
        label={error}
        testid={TESTID_CHAT_ERROR}
      >
        {#snippet actions()}
          {#if canRetryTurn}
            <Button
              type="button"
              onclick={retryTurn}
              testid={TESTID_CHAT_RETRY_TURN}>{LABEL_RETRY}</Button
            >
          {:else if canRetryLoad}
            <Button type="button" onclick={retry}>{LABEL_RETRY}</Button>
          {/if}
          <Button type="button" onclick={dismiss}>{LABEL_DISMISS}</Button>
        {/snippet}
      </StateBlock>
    {/if}

    {#if turnStatus !== null}
      <p
        class="chat__turn-status"
        data-testid={TESTID_CHAT_TURN_STATUS}
        role="status"
      >
        {chatTurnStatusLabel(turnStatus)} · {turnElapsed(elapsedSeconds)}
      </p>
    {/if}
  </div>

  <Composer />
</div>

<style>
  .chat {
    display: flex;
    flex-direction: column;
    flex: 1;
    min-width: 0;
    max-width: 100%;
    min-height: 0;
    overflow: hidden;
  }

  .chat__messages {
    flex: 1;
    overflow-y: auto;
    overflow-x: clip;
    display: flex;
    flex-direction: column;
    min-width: 0;
    max-width: 100%;
    gap: var(--space-4);
    padding: var(--space-4);
  }

  .chat__empty {
    color: var(--muted);
    font-family: var(--font-display);
    font-size: var(--text-sm);
  }

  .chat__turn-status {
    align-self: flex-start;
    color: var(--muted);
    font-family: var(--font-display);
    font-size: var(--text-xs);
    padding: var(--space-2) var(--space-3);
    border: var(--border-width) solid var(--border);
    border-radius: var(--radius);
    background: var(--panel);
  }
</style>
