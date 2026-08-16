<script lang="ts">
  import type { Snippet } from "svelte";
  import { LABEL_CLOSE } from "$lib/common/labels";

  interface Props {
    title: string;
    onClose: () => void;
    testid?: string;
    children: Snippet;
  }

  const { title, onClose, testid, children }: Props = $props();

  function onKeydown(event: KeyboardEvent): void {
    if (event.key === "Escape") {
      onClose();
    }
  }

  // Focus the first real form control when the dialog mounts (skips the close
  // button so the user lands in the form, not on ✕).
  function autofocus(node: HTMLElement): void {
    node.querySelector<HTMLElement>("input, select, textarea")?.focus();
  }
</script>

<svelte:window onkeydown={onKeydown} />

<div class="modal-overlay">
  <!-- The scrim is a real button so backdrop-click-to-close is keyboard- and
       screen-reader-accessible without static-element interaction warnings. -->
  <button
    class="modal-overlay__scrim"
    type="button"
    aria-label={LABEL_CLOSE}
    onclick={onClose}
  ></button>

  <div
    class="modal"
    role="dialog"
    aria-modal="true"
    aria-label={title}
    data-testid={testid}
    use:autofocus
  >
    <div class="modal__head">
      <h2 class="modal__title">{title}</h2>
      <button
        class="modal__close"
        type="button"
        aria-label={LABEL_CLOSE}
        onclick={onClose}>&times;</button
      >
    </div>
    <div class="modal__body">
      {@render children()}
    </div>
  </div>
</div>

<style>
  .modal-overlay {
    position: fixed;
    inset: 0;
    z-index: 100;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: var(--space-4);
  }

  .modal-overlay__scrim {
    position: absolute;
    inset: 0;
    border: none;
    padding: 0;
    cursor: default;
    background: rgba(2, 6, 23, 0.6);
  }

  .modal {
    position: relative;
    z-index: 1;
    width: 70vw;
    max-width: 64rem;
    max-height: calc(100vh - var(--space-8));
    display: flex;
    flex-direction: column;
    background: var(--panel);
    border: var(--border-width) solid var(--border);
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow-lg);
  }

  .modal__head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
    padding: var(--space-4);
    border-bottom: var(--border-width) solid var(--border);
  }

  .modal__title {
    font-size: var(--text-lg);
    font-weight: 600;
  }

  .modal__close {
    background: transparent;
    border: none;
    color: var(--muted);
    cursor: pointer;
    font-size: var(--text-xl);
    line-height: 1;
    padding: 0 var(--space-1);
  }

  .modal__close:hover {
    color: var(--ink);
  }

  .modal__body {
    padding: var(--space-4);
    overflow-y: auto;
  }

  @media (max-width: 640px) {
    /* 70vw reads as cramped on a phone-width viewport — go near-full-width
       instead, matching the overlay's own space-4 padding. */
    .modal {
      width: 100%;
    }
  }
</style>
