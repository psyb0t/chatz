<script lang="ts">
  import { onMount } from "svelte";
  import { ApiError } from "$lib/api/errors";
  import { auth } from "$lib/stores/auth.svelte";
  import { users } from "$lib/stores/users.svelte";
  import Card from "$lib/components/ui/Card.svelte";
  import Panel from "$lib/components/ui/Panel.svelte";
  import Field from "$lib/components/ui/Field.svelte";
  import Button from "$lib/components/ui/Button.svelte";
  import Badge from "$lib/components/ui/Badge.svelte";
  import StateBlock from "$lib/components/ui/StateBlock.svelte";
  import {
    BUTTON_PRIMARY,
    BUTTON_DANGER,
    BADGE_INFO,
    BADGE_NEUTRAL,
    STATE_LOADING,
    STATE_EMPTY,
    STATE_ERROR,
  } from "$lib/components/ui/variants";
  import {
    ADMIN_USERS_TITLE,
    ADMIN_USERS_CREATE_TITLE,
    ADMIN_FORBIDDEN_TITLE,
    ADMIN_FORBIDDEN_BODY,
    LABEL_CREATE,
    LABEL_DELETE,
    STATE_LOADING as COPY_LOADING,
    EMPTY_USERS,
  } from "$lib/common/labels";
  import {
    TESTID_ADMIN_FORBIDDEN,
    TESTID_ADMIN_LOADING,
    TESTID_ADMIN_EMPTY,
    TESTID_ADMIN_ERROR,
    TESTID_USER_CREATE_SUBMIT,
    TESTID_USER_CREATE_USERNAME,
    TESTID_USER_CREATE_PASSWORD,
    TESTID_USER_CREATE_ADMIN,
    TESTID_USER_DELETE,
  } from "$lib/common/test-ids";

  const isAdmin = $derived(auth.user?.isAdmin === true);

  // Load the user list once, only for an admin. A non-admin never triggers the
  // (server-rejected) request.
  onMount(() => {
    if (isAdmin && !users.loaded) {
      void users.load();
    }
  });

  let username = $state("");
  let password = $state("");
  let makeAdmin = $state(false);
  let formError = $state("");
  let submitting = $state(false);
  let busyId = $state<string | null>(null);

  async function onCreate(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    formError = "";
    submitting = true;

    try {
      await users.create({ username, password, isAdmin: makeAdmin });
      username = "";
      password = "";
      makeAdmin = false;
    } catch (err) {
      formError =
        err instanceof ApiError ? err.message : "create failed, try again";
    } finally {
      submitting = false;
    }
  }

  async function onDelete(userId: string): Promise<void> {
    busyId = userId;

    try {
      await users.remove(userId);
    } catch (err) {
      formError =
        err instanceof ApiError ? err.message : "delete failed, try again";
    } finally {
      busyId = null;
    }
  }
</script>

{#if !isAdmin}
  <div class="admin admin--forbidden">
    <Panel shadow pad="lg">
      <div class="forbidden" data-testid={TESTID_ADMIN_FORBIDDEN}>
        <h1 class="forbidden__title">{ADMIN_FORBIDDEN_TITLE}</h1>
        <p class="forbidden__body">{ADMIN_FORBIDDEN_BODY}</p>
      </div>
    </Panel>
  </div>
{:else}
  <div class="admin">
    <h1 class="admin__title">{ADMIN_USERS_TITLE}</h1>

    <div class="admin__list">
      {#if users.loading && users.list.length === 0}
        <StateBlock
          variant={STATE_LOADING}
          label={COPY_LOADING}
          testid={TESTID_ADMIN_LOADING}
        />
      {:else if users.error !== null && users.list.length === 0}
        <StateBlock
          variant={STATE_ERROR}
          label={users.error}
          testid={TESTID_ADMIN_ERROR}
        />
      {:else if users.list.length === 0}
        <StateBlock
          variant={STATE_EMPTY}
          label={EMPTY_USERS}
          testid={TESTID_ADMIN_EMPTY}
        />
      {:else}
        <ul class="admin__actions">
          {#each users.list as u (u.id)}
            <li class="admin__action-row">
              <span class="admin__action-name">
                {u.username}
                {#if u.isAdmin}
                  <Badge label="admin" variant={BADGE_INFO} />
                {:else}
                  <Badge label="user" variant={BADGE_NEUTRAL} />
                {/if}
              </span>
              <span class="admin__action-created">
                {new Date(u.createdAt).toLocaleString()}
              </span>
              {#if u.id !== auth.user?.id}
                <Button
                  variant={BUTTON_DANGER}
                  testid={TESTID_USER_DELETE}
                  disabled={busyId === u.id}
                  onclick={() => onDelete(u.id)}
                >
                  {LABEL_DELETE}
                </Button>
              {/if}
            </li>
          {/each}
        </ul>
      {/if}
    </div>

    <Card title={ADMIN_USERS_CREATE_TITLE}>
      <form class="admin__form" onsubmit={onCreate}>
        <Field label="Username">
          {#snippet control()}
            <input
              class="admin__input"
              type="text"
              autocomplete="off"
              data-testid={TESTID_USER_CREATE_USERNAME}
              bind:value={username}
              required
            />
          {/snippet}
        </Field>

        <Field label="Password" error={formError || null}>
          {#snippet control()}
            <input
              class="admin__input"
              type="password"
              autocomplete="new-password"
              data-testid={TESTID_USER_CREATE_PASSWORD}
              bind:value={password}
              required
            />
          {/snippet}
        </Field>

        <label class="admin__check">
          <input
            type="checkbox"
            data-testid={TESTID_USER_CREATE_ADMIN}
            bind:checked={makeAdmin}
          />
          <span>Admin</span>
        </label>

        <Button
          variant={BUTTON_PRIMARY}
          type="submit"
          testid={TESTID_USER_CREATE_SUBMIT}
          disabled={submitting}
        >
          {submitting ? "Working…" : LABEL_CREATE}
        </Button>
      </form>
    </Card>
  </div>
{/if}

<style>
  .admin {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
    padding: var(--space-6);
    overflow-y: auto;
  }

  .admin--forbidden {
    align-items: center;
    justify-content: center;
    flex: 1;
  }

  .admin__title {
    font-size: var(--text-2xl);
  }

  .admin__list {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .admin__actions {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    list-style: none;
  }

  .admin__action-row {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    border: var(--border-width) solid var(--border);
    background: var(--panel);
    padding: var(--space-2) var(--space-3);
  }

  .admin__action-name {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-family: var(--font-mono);
    font-size: var(--text-sm);
  }

  .admin__action-created {
    margin-left: auto;
    color: var(--muted);
    font-family: var(--font-mono);
    font-size: var(--text-sm);
  }

  .admin__form {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .admin__input {
    font-family: var(--font-mono);
    font-size: var(--text-sm);
  }

  .admin__form :global(button[type="submit"]) {
    align-self: flex-start;
  }

  .admin__check {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-family: var(--font-display);
    font-size: var(--text-sm);
    font-weight: 500;
    color: var(--muted);
  }

  .forbidden {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    text-align: center;
  }

  .forbidden__title {
    font-size: var(--text-2xl);
    color: var(--crit);
  }

  .forbidden__body {
    color: var(--muted);
    font-family: var(--font-mono);
    font-size: var(--text-sm);
  }
</style>
