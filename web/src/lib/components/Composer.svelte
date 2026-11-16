<script lang="ts">
  import { models } from "#lib/stores/models.svelte.js";
  import { conversation } from "#lib/stores/conversation.svelte.js";
  import {
    previewChatContext,
    type Model,
    type PromptContextPreview,
  } from "#lib/api/client.js";
  import {
    TESTID_COMPOSER,
    TESTID_COMPOSER_SEND,
    TESTID_COMPOSER_STOP,
    TESTID_CHAT_SETTINGS_TOGGLE,
    TESTID_CONTEXT_METER,
  } from "#lib/common/test-ids.js";
  import {
    A11Y_COMPOSER_INPUT,
    A11Y_CHAT_SETTINGS,
    A11Y_SEND,
    A11Y_STOP,
    COMPOSER_PLACEHOLDER,
    COMPOSER_HINT,
    COMPOSER_NO_MODELS,
    COMPOSER_NO_UPSTREAM,
    contextBreakdown,
    contextMeter,
    contextOmitted,
  } from "#lib/common/labels.js";
  import ChatSettings from "#lib/components/ChatSettings.svelte";
  import ChatMcpPicker from "#lib/components/ChatMcpPicker.svelte";
  import ModelPicker from "#lib/components/ModelPicker.svelte";
  import ReasoningPicker from "#lib/components/ReasoningPicker.svelte";
  import Icon from "#lib/components/ui/Icon.svelte";

  let selectedModel = $state("");
  let draft = $state("");
  let inputEl: HTMLTextAreaElement | undefined = $state();
  let settingsOpen = $state(false);
  let contextPreview = $state<PromptContextPreview | null>(null);
  const CONTEXT_PREVIEW_DELAY_MS = 150;
  const TOKENS_PER_K = 1000;
  const METER_RADIUS = 7;
  const METER_CIRCUMFERENCE = 2 * Math.PI * METER_RADIUS;
  const METER_WARN_RATIO = 0.8;
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

  // Auto-grow the textarea from one row up to the CSS max-height, then it
  // scrolls. Reset to `auto` first so it can shrink when text is deleted or
  // the draft is cleared after send. Runs on every draft change.
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
  const activeChatId = $derived(conversation.chatId);
  const streaming = $derived(conversation.streaming);
  const trimmed = $derived(draft.trim());
  const canSend = $derived(
    !streaming && trimmed !== "" && hasModels && selectedModel !== "",
  );
  const contextRatio = $derived(
    contextPreview === null || contextPreview.budgetTokens === 0
      ? 0
      : Math.min(contextPreview.totalTokens / contextPreview.budgetTokens, 1),
  );

  function compactTokens(tokens: number): string {
    return tokens >= TOKENS_PER_K
      ? `${(tokens / TOKENS_PER_K).toFixed(1).replace(/\.0$/, "")}k`
      : String(tokens);
  }

  // The meter deliberately asks the backend after a short idle delay. Only the
  // backend has the durable transcript and the same tokenizer/selection path
  // the next streamed turn uses.
  //
  // The in-flight value is deliberately KEPT rather than cleared, so the meter
  // does not vanish on every keystroke. It is only discarded when the chat
  // changes, where the old numbers would be wrong rather than merely late.
  $effect(() => {
    const chatId = conversation.chatId;
    const message = draft;
    void conversation.streaming;
    void conversation.settings;

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

  // Enter sends; Shift+Enter inserts a newline. An IME composition's Enter
  // confirms the composition and must not send.
  function onKeydown(event: KeyboardEvent): void {
    if (event.key === "Enter" && !event.shiftKey && !event.isComposing) {
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

  // setDraft fills the message box, for the empty-state suggestions.
  export function setDraft(text: string): void {
    draft = text;
    queueMicrotask(() => inputEl?.focus());
  }
</script>

<form
  class="composer"
  id="composer"
  data-testid={TESTID_COMPOSER}
  onsubmit={onSubmit}
>
  <div class="composer__box">
    <textarea
      class="composer__input"
      id="composer-input"
      bind:this={inputEl}
      aria-label={A11Y_COMPOSER_INPUT}
      placeholder={COMPOSER_PLACEHOLDER}
      rows="1"
      bind:value={draft}
      onkeydown={onKeydown}
      disabled={streaming}></textarea>

    <div class="composer__bar">
      <div class="composer__group">
        <!-- Settings open even before the first message; the panel saves to
             the chat the route already created. -->
        <div class="composer__settings-anchor">
          <button
            class="composer__icon-btn"
            type="button"
            onclick={toggleSettings}
            aria-label={A11Y_CHAT_SETTINGS}
            aria-pressed={settingsOpen}
            title={A11Y_CHAT_SETTINGS}
            data-testid={TESTID_CHAT_SETTINGS_TOGGLE}
          >
            <Icon name="sliders" size={17} />
          </button>
          {#if settingsOpen}
            <ChatSettings
              model={selectedModelInfo}
              onClose={() => (settingsOpen = false)}
            />
          {/if}
        </div>

        <ReasoningPicker model={selectedModelInfo} disabled={streaming} />

        {#if activeChatId !== null}
          <ChatMcpPicker chatId={activeChatId} />
        {/if}
      </div>

      <div class="composer__group composer__group--end">
        {#if contextPreview !== null}
          <span
            class="composer__meter"
            class:composer__meter--warn={contextRatio >= METER_WARN_RATIO}
            data-testid={TESTID_CONTEXT_METER}
            title={[
              contextMeter(
                contextPreview.totalTokens,
                contextPreview.budgetTokens,
                contextPreview.availableTokens,
              ),
              contextBreakdown(
                contextPreview.systemTokens,
                contextPreview.historyTokens,
                contextPreview.currentMessageTokens,
              ),
              contextPreview.omittedTurns > 0
                ? contextOmitted(
                    contextPreview.omittedTurns,
                    contextPreview.omittedMessages,
                  )
                : "",
            ]
              .filter((line) => line !== "")
              .join("\n")}
          >
            <svg width="18" height="18" viewBox="0 0 18 18" aria-hidden="true">
              <circle
                cx="9"
                cy="9"
                r={METER_RADIUS}
                fill="none"
                stroke="var(--border-strong)"
                stroke-width="2"
              />
              <circle
                cx="9"
                cy="9"
                r={METER_RADIUS}
                fill="none"
                stroke="currentColor"
                stroke-width="2"
                stroke-linecap="round"
                stroke-dasharray={METER_CIRCUMFERENCE}
                stroke-dashoffset={METER_CIRCUMFERENCE * (1 - contextRatio)}
                transform="rotate(-90 9 9)"
              />
            </svg>
            <span class="composer__meter-text">
              {compactTokens(contextPreview.totalTokens)} / {compactTokens(
                contextPreview.budgetTokens,
              )}
            </span>
            <span class="sr-only">
              {contextMeter(
                contextPreview.totalTokens,
                contextPreview.budgetTokens,
                contextPreview.availableTokens,
              )}
            </span>
          </span>
        {/if}

        {#if hasModels}
          <ModelPicker
            models={models.list}
            bind:value={selectedModel}
            disabled={streaming}
          />
        {:else}
          <span class="composer__nomodel">{COMPOSER_NO_MODELS}</span>
        {/if}

        {#if streaming}
          <button
            class="composer__send composer__send--stop"
            type="button"
            onclick={stop}
            aria-label={A11Y_STOP}
            title={A11Y_STOP}
            data-testid={TESTID_COMPOSER_STOP}
          >
            <Icon name="stop" size={14} strokeWidth={2.4} />
          </button>
        {:else}
          <button
            class="composer__send"
            type="submit"
            disabled={!canSend}
            aria-label={A11Y_SEND}
            title={A11Y_SEND}
            data-testid={TESTID_COMPOSER_SEND}
          >
            <Icon name="arrow-up" size={17} strokeWidth={2.2} />
          </button>
        {/if}
      </div>
    </div>
  </div>

  <p class="composer__hint">
    {hasModels ? COMPOSER_HINT : COMPOSER_NO_UPSTREAM}
  </p>
</form>

<style>
  .composer {
    position: relative;
    width: 100%;
    max-width: calc(var(--reading-width) + 2 * var(--space-4));
    margin: 0 auto;
    padding: 0 var(--space-4) var(--space-3);
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .composer__box {
    display: flex;
    flex-direction: column;
    border: var(--border-width) solid var(--border-strong);
    border-radius: var(--radius-2xl);
    background: var(--surface);
    box-shadow: var(--shadow-composer);
    transition:
      border-color var(--dur-fast) var(--ease),
      box-shadow var(--dur-fast) var(--ease);
  }

  .composer__box:focus-within {
    border-color: color-mix(in srgb, var(--ink) 25%, var(--border-strong));
  }

  .composer__input {
    width: 100%;
    min-width: 0;
    border: none;
    background: transparent;
    resize: none;
    overflow-y: auto;
    /* One row by default, growing to about eight before it scrolls (JS sets
       the height; this clamps it). */
    line-height: 1.5;
    max-height: calc(12em + var(--space-4));
    padding: var(--space-4) var(--space-4) var(--space-2);
    font-size: var(--text-prose);
  }

  .composer__input::placeholder {
    color: var(--faint);
  }

  /* The global focus ring would light up the textarea inside the rounded box;
     the box's own focus-within border does that job. */
  .composer__input:focus,
  .composer__input:focus-visible {
    outline: none;
    border-color: transparent;
    box-shadow: none;
  }

  .composer__bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
    padding: var(--space-1) var(--space-2) var(--space-2);
    min-width: 0;
  }

  .composer__group {
    display: flex;
    align-items: center;
    gap: 2px;
    min-width: 0;
  }

  .composer__group--end {
    gap: var(--space-1);
  }

  .composer__settings-anchor {
    position: relative;
    display: inline-flex;
  }

  .composer__icon-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: var(--control-sm);
    height: var(--control-sm);
    padding: 0;
    background: transparent;
    border: none;
    border-radius: var(--radius-pill);
    color: var(--muted);
  }

  .composer__icon-btn:hover:not(:disabled),
  .composer__icon-btn[aria-pressed="true"] {
    background: var(--fill-hover);
    color: var(--ink);
  }

  .composer__meter {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 0 var(--space-1);
    color: var(--muted);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    font-variant-numeric: tabular-nums;
    cursor: default;
  }

  .composer__meter--warn {
    color: var(--warn);
  }

  .composer__send {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 34px;
    height: 34px;
    flex-shrink: 0;
    padding: 0;
    border: none;
    border-radius: 50%;
    background: var(--primary);
    color: var(--on-primary);
  }

  .composer__send:hover:not(:disabled) {
    background: var(--primary-hover);
  }

  .composer__send:disabled {
    background: var(--panel-2);
    color: var(--disabled);
    opacity: 1;
  }

  .composer__nomodel {
    color: var(--faint);
    font-size: var(--text-sm);
  }

  .composer__hint {
    color: var(--faint);
    font-size: var(--text-xs);
    text-align: center;
  }

  @media (max-width: 640px) {
    .composer {
      padding: 0 var(--space-2) var(--space-2);
    }

    .composer__meter-text,
    .composer__hint {
      display: none;
    }
  }
</style>
