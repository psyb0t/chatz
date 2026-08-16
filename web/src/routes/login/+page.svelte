<script lang="ts">
  import { ApiError } from "$lib/api/errors";
  import { auth } from "$lib/stores/auth.svelte";
  import Panel from "$lib/components/ui/Panel.svelte";
  import Field from "$lib/components/ui/Field.svelte";
  import Button from "$lib/components/ui/Button.svelte";
  import { BUTTON_PRIMARY } from "$lib/components/ui/variants";

  let username = $state("");
  let password = $state("");
  let error = $state("");
  let submitting = $state(false);

  async function onSubmit(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    error = "";
    submitting = true;

    try {
      await auth.login({ username, password });
    } catch (err) {
      error =
        err instanceof ApiError ? err.message : "sign in failed, try again";
    } finally {
      submitting = false;
    }
  }
</script>

<div class="gate">
  <Panel shadow pad="lg">
    <form class="gate__form" onsubmit={onSubmit}>
      <h1 class="gate__title">Sign In</h1>

      <Field label="Username">
        {#snippet control()}
          <input
            class="gate__input"
            type="text"
            autocomplete="username"
            data-testid="login-username"
            bind:value={username}
            required
          />
        {/snippet}
      </Field>

      <Field label="Password" {error} errorTestid="login-error">
        {#snippet control()}
          <input
            class="gate__input"
            type="password"
            autocomplete="current-password"
            data-testid="login-password"
            bind:value={password}
            required
          />
        {/snippet}
      </Field>

      <Button
        variant={BUTTON_PRIMARY}
        type="submit"
        testid="login-submit"
        disabled={submitting}
      >
        {submitting ? "Working…" : "Sign In"}
      </Button>
    </form>
  </Panel>
</div>

<style>
  .gate {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 100vh;
    padding: var(--space-4);
  }

  :global(.gate > .panel) {
    width: 100%;
    max-width: 24rem;
  }

  .gate__form {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .gate__title {
    font-size: var(--text-2xl);
  }

  .gate__input {
    font-family: var(--font-mono);
    font-size: var(--text-sm);
  }
</style>
