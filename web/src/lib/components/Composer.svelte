<script lang="ts">
  import { models } from "$lib/stores/models.svelte";
  import { conversation } from "$lib/stores/conversation.svelte";
  import {
    previewChatContext,
    type Model,
    type PromptContextPreview,
  } from "$lib/api/client";
  import {
    TESTID_COMPOSER,
    TESTID_COMPOSER_SEND,
    TESTID_COMPOSER_STOP,
    TESTID_CHAT_SETTINGS_TOGGLE,
    TESTID_CONTEXT_METER,
  } from "$lib/common/test-ids";
  import {
    A11Y_COMPOSER_INPUT,
    A11Y_CHAT_SETTINGS,
    contextBreakdown,
    contextMeter,
    contextOmitted,
  } from "$lib/common/labels";
  import Button from "$lib/components/ui/Button.svelte";
  import ChatSettings from "$lib/components/ChatSettings.svelte";
  import ChatMcpPicker from "$lib/components/ChatMcpPicker.svelte";
  import ModelPicker from "$lib/components/ModelPicker.svelte";
  import { BUTTON_PRIMARY, BUTTON_DANGER } from "$lib/components/ui/variants";

  let selectedModel = $state("");
  let draft = $state("");
  let inputEl: HTMLTextAreaElement | undefined = $state();
  let settingsOpen = $state(false);
  let contextPreview = $state<PromptContextPreview | null>(null);
  const CONTEXT_PREVIEW_DELAY_MS = 150;
  // Plain (non-reactive): which chat `contextPreview` was computed for, so the
  // meter is only blanked on a real chat switch. See the preview effect below.
  let previewChatId: string | null = null;
  // Plain (non-reactive): only re-sync the picker when the loaded chat's model
  // actually changes, so the user can still change the picker afterwards.
  let lastSyncedModel: string | null = null;

  // Default the picker to the configured instance model once the list arrives.
  $effect(() => {
    if (selectedModel === "" && models.list.length > 0) {
      selectedModel =
        models.list.find((model) => model.default)?.id ?? models.list[0].id;
    }
  });

  // Reflect the active chat's model in the picker when a chat is loaded (the
  // model a continued turn uses), so switching chats preserves the model shown.
  $effect(() => {
    const chatModel = conversation.model;
    if (
      chatModel !== null &&
      chatModel !== "" &&
      chatModel !== lastSyncedModel
    ) {
      lastSyncedModel = chatModel;
      selectedModel = chatModel;
    }
  });

  // Auto-grow the textarea from one row up to the CSS max-height (~4 rows, then
  // it scrolls). Reset to `auto` first so it can shrink when text is deleted or
  // the draft is cleared after send. Runs on every draft change (typing + clear).
  $effect(() => {
    void draft;
    const el = inputEl;
    if (el === undefined) {
      return;
    }

    el.style.height = "auto";
    el.style.height = `${el.scrollHeight}px`;
  });

  const hasModels = $derived(models.list.length > 0);
  const selectedModelInfo = $derived<Model | undefined>(
    models.list.find((model) => model.id === selectedModel),
  );
  // Non-null chat id for the MCP picker prop (the {#if} guard narrows it).
  const activeChatId = $derived(conversation.chatId);
  const streaming = $derived(conversation.streaming);
  const trimmed = $derived(draft.trim());
  const canSend = $derived(
    !streaming && trimmed !== "" && hasModels && selectedModel !== "",
  );

  // The meter deliberately asks the backend after a short idle delay. Only the
  // backend has the durable transcript and the same tokenizer/selection path
  // the next streamed turn uses.
  //
  // The in-flight value is deliberately KEPT rather than cleared. The bar is
  // gated on `contextPreview !== null`, so nulling it on every keystroke made
  // the whole meter vanish while typing and reappear on idle. A slightly stale
  // count is better than no count. It is only discarded when the chat changes,
  // where the old numbers would be wrong rather than merely late.
  $effect(() => {
    const chatId = conversation.chatId;
    const message = draft;
    void conversation.streaming;

    if (chatId !== previewChatId) {
      previewChatId = chatId;
      contextPreview = null;
    }

    if (chatId === null) {
      return;
    }

    let active = true;
    const timer = setTimeout(() => {
      void previewChatContext(chatId, message)
        .then((preview) => {
          if (active) {
            contextPreview = preview;
          }
        })
        .catch(() => {
          // A preview is informative; failure must never interrupt typing.
        });
    }, CONTEXT_PREVIEW_DELAY_MS);

    return () => {
      active = false;
      clearTimeout(timer);
    };
  });

  async function submit(): Promise<void> {
    if (!canSend) {
      return;
    }

    const text = draft;
    draft = "";
    await conversation.send(text, selectedModel);
  }

  // Enter sends; Shift+Enter inserts a newline (default textarea behavior).
  function onKeydown(event: KeyboardEvent): void {
    if (event.key === "Enter" && !event.shiftKey) {
      event.preventDefault();
      void submit();
    }
  }

  function onSubmit(event: SubmitEvent): void {
    event.preventDefault();
    void submit();
  }

  function stop(): void {
    conversation.stop();
  }

  function toggleSettings(): void {
    settingsOpen = !settingsOpen;
  }

  export function focusInput(): void {
    inputEl?.focus();
  }
</script>

<form
  class="composer"
  id="composer"
  data-testid={TESTID_COMPOSER}
  onsubmit={onSubmit}
>
  <!-- One bordered box, grid-laid-out so mobile can reflow it: desktop is one
       row (input flexes, the four controls ride the right edge); mobile is two
       rows (options row: gear/mcp/model; input row: textarea + send/stop) so
       the textarea keeps a usable width on narrow phones instead of the
       controls crowding it out. See the mobile media query below. -->
  <div class="composer__box">
    <textarea
      class="composer__input"
      id="composer-input"
      bind:this={inputEl}
      aria-label={A11Y_COMPOSER_INPUT}
      placeholder="type&hellip;"
      rows="1"
      bind:value={draft}
      onkeydown={onKeydown}
      disabled={streaming}
    ></textarea>

    <!-- Settings are available even before a chat exists: on a new chat they
         buffer and get pushed to the chat the first message creates. -->
    <div class="composer__opt composer__opt--gear composer__settings-anchor">
      <button
        class="composer__gear"
        type="button"
        onclick={toggleSettings}
        aria-label={A11Y_CHAT_SETTINGS}
        aria-pressed={settingsOpen}
        data-testid={TESTID_CHAT_SETTINGS_TOGGLE}>&#9881;</button
      >
      {#if settingsOpen}
        <ChatSettings
          model={selectedModelInfo}
          onClose={() => (settingsOpen = false)}
        />
      {/if}
    </div>

    {#if activeChatId !== null}
      <div class="composer__opt composer__opt--mcp">
        <ChatMcpPicker chatId={activeChatId} />
      </div>
    {/if}

    <div class="composer__opt composer__opt--model">
      {#if hasModels}
        <ModelPicker
          models={models.list}
          bind:value={selectedModel}
          disabled={streaming}
        />
      {:else}
        <span class="composer__nomodel">no models</span>
      {/if}
    </div>

    <div class="composer__opt composer__opt--send">
      {#if streaming}
        <Button
          variant={BUTTON_DANGER}
          type="button"
          onclick={stop}
          testid={TESTID_COMPOSER_STOP}>Stop</Button
        >
      {:else}
        <Button
          variant={BUTTON_PRIMARY}
          type="submit"
          disabled={!canSend}
          testid={TESTID_COMPOSER_SEND}>Send</Button
        >
      {/if}
    </div>
  </div>

  {#if contextPreview !== null}
    <div class="composer__context" data-testid={TESTID_CONTEXT_METER}>
      <span>
        {contextMeter(
          contextPreview.totalTokens,
          contextPreview.budgetTokens,
          contextPreview.availableTokens,
        )}
      </span>
      <span class="composer__context-detail">
        {contextBreakdown(
          contextPreview.systemTokens,
          contextPreview.historyTokens,
          contextPreview.currentMessageTokens,
        )}
      </span>
      {#if contextPreview.omittedTurns > 0}
        <span class="composer__context-omitted">
          {contextOmitted(
            contextPreview.omittedTurns,
            contextPreview.omittedMessages,
          )}
        </span>
      {/if}
    </div>
  {/if}

  {#if !hasModels}
    <span class="composer__hint"
      >no upstream configured &mdash; wire one up to chat</span
    >
  {/if}
</form>

<style>
  .composer {
    position: relative;
    padding: var(--space-4);
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .composer__box {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding-right: var(--space-2);
    border: var(--border-width) solid var(--border-strong);
    border-radius: var(--radius);
    background: var(--panel);
    box-shadow: var(--shadow-sm);
  }

  .composer__box:focus-within {
    border-color: var(--accent);
  }

  .composer__input {
    flex: 1;
    min-width: 0;
    box-sizing: border-box;
    border: none;
    background: transparent;
    resize: none;
    overflow-y: auto;
    /* One row by default, growing to four before it scrolls (JS sets height;
       this clamps it): 4 lines at line-height 1.5 = 6em, plus the padding. */
    line-height: 1.5;
    max-height: calc(6em + var(--space-3) * 2);
    padding: var(--space-3);
    font-family: var(--font-display);
    font-size: var(--text-sm);
  }

  /* The global input/textarea focus styles (outline + border + accent-soft
     box-shadow glow) would light up the textarea itself, poking past the box's
     rounded border. Suppress all of them and let .composer__box:focus-within
     highlight the whole box border instead. */
  .composer__input:focus,
  .composer__input:focus-visible {
    outline: none;
    border-color: transparent;
    box-shadow: none;
  }

  /* Each option (gear/mcp/model/send) is a direct .composer__box child so
     the mobile media query below can independently place them via
     grid-area — a flat sibling list, not a nested controls wrapper. */
  .composer__opt {
    display: inline-flex;
    align-items: center;
  }

  .composer__gear {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: var(--border-width) solid transparent;
    border-radius: var(--radius);
    color: var(--muted);
    cursor: pointer;
    line-height: 1;
    padding: var(--space-1) var(--space-2);
    font-size: var(--text-sm);
    transition:
      background-color 0.12s ease,
      color 0.12s ease;
  }

  .composer__gear:hover {
    background: var(--panel-2);
    color: var(--ink);
  }

  .composer__gear[aria-pressed="true"] {
    background: var(--panel-2);
    color: var(--accent);
  }

  .composer__settings-anchor {
    position: relative;
    display: inline-flex;
  }

  .composer__nomodel {
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    color: var(--muted);
  }

  .composer__hint {
    font-size: var(--text-xs);
    color: var(--muted);
  }

  .composer__context {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-1) var(--space-2);
    color: var(--muted);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
  }

  .composer__context-detail {
    color: var(--muted);
  }

  .composer__context-omitted {
    color: var(--warn);
  }

  @media (max-width: 1020px) {
    /* Two rows instead of one: an options row (gear/mcp/model) above an input
       row (textarea + send/stop) — below this width, cramming all four
       controls onto the textarea's row left almost no room to type. Grid
       areas let each control be placed independently per row without
       touching the component markup. Wider than the sidebar's own 640px
       mobile breakpoint on purpose: the composer gets cramped well before the
       sidebar needs to collapse into a drawer. */
    .composer__box {
      display: grid;
      grid-template-columns: auto auto 1fr auto;
      grid-template-areas:
        "gear mcp model model"
        "input input input send";
      align-items: center;
      gap: var(--space-2);
      padding: var(--space-2);
    }

    .composer__input {
      grid-area: input;
      padding: var(--space-2);
    }

    .composer__opt--gear {
      grid-area: gear;
    }

    .composer__opt--mcp {
      grid-area: mcp;
    }

    .composer__opt--model {
      grid-area: model;
      min-width: 0;
      justify-self: stretch;
    }

    .composer__opt--send {
      grid-area: send;
    }
  }
</style>
