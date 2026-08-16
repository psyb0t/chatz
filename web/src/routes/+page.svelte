<script lang="ts">
  import { chats } from "$lib/stores/chats.svelte";
  import { conversation } from "$lib/stores/conversation.svelte";
  import StateBlock from "$lib/components/ui/StateBlock.svelte";
  import Button from "$lib/components/ui/Button.svelte";
  import { STATE_ERROR, STATE_LOADING } from "$lib/components/ui/variants";
  import {
    TESTID_NEW_CHAT_LOADING,
    TESTID_NEW_CHAT_ERROR,
  } from "$lib/common/test-ids";
  import { STATE_STARTING_CHAT, LABEL_RETRY } from "$lib/common/labels";

  // The root route is a pure redirector: it resolves the caller's reusable
  // empty chat (creating one if none exists — see chats.goToNewChat) and
  // navigates to /chat/{id}. Every chat page always has a real chat behind
  // it, so nothing composer-related renders here.
  $effect(() => {
    conversation.reset();
    void chats.goToNewChat();
  });
</script>

{#if chats.error !== null}
  <StateBlock
    variant={STATE_ERROR}
    label={chats.error}
    testid={TESTID_NEW_CHAT_ERROR}
  >
    {#snippet actions()}
      <Button type="button" onclick={() => chats.goToNewChat()}
        >{LABEL_RETRY}</Button
      >
    {/snippet}
  </StateBlock>
{:else}
  <StateBlock
    variant={STATE_LOADING}
    label={STATE_STARTING_CHAT}
    testid={TESTID_NEW_CHAT_LOADING}
  />
{/if}
