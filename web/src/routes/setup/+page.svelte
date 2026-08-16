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
      await auth.setup({ username, password });
    } catch (err) {
      error = err instanceof ApiError ? err.message : "setup failed, try again";
    } finally {
      submitting = false;
    }
  }
</script>

<div class="gate">
  <Panel shadow pad="lg">
    <form class="gate__form" onsubmit={onSubmit}>
      <div class="gate__intro">
        <h1 class="gate__title">Create Admin</h1>
        <p class="gate__sub">First-run bootstrap. This account is the admin.</p>
      </div>

      <Field label="Username">
        {#snippet control()}
          <input
            class="gate__input"
            type="text"
            autocomplete="username"
            data-testid="setup-username"
            bind:value={username}
            required
          />
        {/snippet}
      </Field>

      <Field label="Password" {error} errorTestid="setup-error">
        {#snippet control()}
          <input
            class="gate__input"
            type="password"
            autocomplete="new-password"
            data-testid="setup-password"
            bind:value={password}
            required
          />
        {/snippet}
      </Field>

      <Button
        variant={BUTTON_PRIMARY}
        type="submit"
        testid="setup-submit"
        disabled={submitting}
      >
        {submitting ? "Working…" : "Create Admin"}
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

  .gate__intro {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .gate__title {
    font-size: var(--text-2xl);
  }

  .gate__sub {
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    color: var(--muted);
  }

  .gate__input {
    font-family: var(--font-mono);
    font-size: var(--text-sm);
  }
</style>
