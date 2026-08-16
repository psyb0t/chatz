<script lang="ts">
  import { page } from "$app/state";
  import ChatPane from "$lib/components/ChatPane.svelte";
  import { conversation } from "$lib/stores/conversation.svelte";

  const chatId = $derived(page.params.chatId ?? "");

  // Load the chat's history when the route points at a different chat than the
  // store already holds. When we arrived here from a just-created chat (send()
  // adopted the id + navigated mid-stream), chatId already matches — so we do
  // NOT reload and clobber the live stream.
  $effect(() => {
    const id = chatId;
    if (id === "" || conversation.chatId === id) {
      return;
    }

    void conversation.load(id);
  });
</script>

<ChatPane />
