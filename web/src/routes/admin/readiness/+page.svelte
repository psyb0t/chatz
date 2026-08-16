<script lang="ts">
  import { onMount } from "svelte";
  import { ApiError } from "$lib/api/errors";
  import { getAdminReadiness, type AdminReadiness } from "$lib/api/client";
  import { auth } from "$lib/stores/auth.svelte";
  import Card from "$lib/components/ui/Card.svelte";
  import Panel from "$lib/components/ui/Panel.svelte";
  import Button from "$lib/components/ui/Button.svelte";
  import Badge from "$lib/components/ui/Badge.svelte";
  import StateBlock from "$lib/components/ui/StateBlock.svelte";
  import {
    BADGE_CRIT,
    BADGE_NEUTRAL,
    BADGE_OK,
    BADGE_WARN,
    STATE_ERROR,
    STATE_LOADING,
    type BadgeVariant,
  } from "$lib/components/ui/variants";
  import {
    ADMIN_FORBIDDEN_BODY,
    ADMIN_FORBIDDEN_TITLE,
    ADMIN_READINESS_BACKUP,
    ADMIN_READINESS_NONE,
    ADMIN_READINESS_NOT_RECORDED,
    ADMIN_READINESS_RUNTIME,
    ADMIN_READINESS_TITLE,
    ADMIN_READINESS_UPSTREAMS,
    LABEL_REFRESH,
    STATE_LOADING as COPY_LOADING,
  } from "$lib/common/labels";
  import {
    TESTID_ADMIN_ERROR,
    TESTID_ADMIN_FORBIDDEN,
    TESTID_ADMIN_LOADING,
    TESTID_ADMIN_READINESS,
    TESTID_ADMIN_READINESS_CONTENT,
  } from "$lib/common/test-ids";

  const isAdmin = $derived(auth.user?.isAdmin === true);

  let readiness = $state<AdminReadiness | null>(null);
  let loading = $state(false);
  let error = $state("");

  function badgeForBackup(
    state: AdminReadiness["backup"]["state"],
  ): BadgeVariant {
    if (state === "current") {
      return BADGE_OK;
    }

    if (state === "stale") {
      return BADGE_WARN;
    }

    if (state === "not_recorded") {
      return BADGE_NEUTRAL;
    }

    return BADGE_CRIT;
  }

  function badgeForUpstream(
    state: AdminReadiness["upstreams"][number]["state"],
  ): BadgeVariant {
    if (state === "healthy") {
      return BADGE_OK;
    }

    if (state === "unknown") {
      return BADGE_NEUTRAL;
    }

    return BADGE_WARN;
  }

  function formatTime(value: string | null | undefined): string {
    return value === undefined || value === null
      ? "—"
      : new Date(value).toLocaleString();
  }

  async function load(): Promise<void> {
    loading = true;
    error = "";

    try {
      readiness = await getAdminReadiness();
    } catch (err) {
      error =
        err instanceof ApiError ? err.message : "readiness request failed";
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    if (isAdmin) {
      void load();
    }
  });
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
  <div class="admin" data-testid={TESTID_ADMIN_READINESS}>
    <div class="admin__heading">
      <h1 class="admin__title">{ADMIN_READINESS_TITLE}</h1>
      <Button onclick={() => void load()} disabled={loading}>
        {LABEL_REFRESH}
      </Button>
    </div>

    {#if loading && readiness === null}
      <StateBlock
        variant={STATE_LOADING}
        label={COPY_LOADING}
        testid={TESTID_ADMIN_LOADING}
      />
    {:else if error !== "" && readiness === null}
      <StateBlock
        variant={STATE_ERROR}
        label={error}
        testid={TESTID_ADMIN_ERROR}
      />
    {:else if readiness !== null}
      <div class="admin__grid" data-testid={TESTID_ADMIN_READINESS_CONTENT}>
        <Card title={ADMIN_READINESS_RUNTIME}>
          <dl class="readiness__facts">
            <div>
              <dt>Version</dt>
              <dd>{readiness.appVersion}</dd>
            </div>
            {#if readiness.commit}
              <div>
                <dt>Commit</dt>
                <dd>{readiness.commit}</dd>
              </div>
            {/if}
            <div>
              <dt>Database</dt>
              <dd>{readiness.databaseDriver}</dd>
            </div>
            <div>
              <dt>Migration</dt>
              <dd>{readiness.migrationVersion}</dd>
            </div>
            <div>
              <dt>Migration state</dt>
              <dd>
                <Badge
                  label={readiness.migrationDirty ? "dirty" : "clean"}
                  variant={readiness.migrationDirty ? BADGE_CRIT : BADGE_OK}
                />
              </dd>
            </div>
          </dl>
        </Card>

        <Card title={ADMIN_READINESS_BACKUP}>
          <div class="readiness__backup">
            <Badge
              label={readiness.backup.state}
              variant={badgeForBackup(readiness.backup.state)}
            />
            {#if readiness.backup.state === "not_recorded"}
              <p>{ADMIN_READINESS_NOT_RECORDED}</p>
            {/if}
            <dl class="readiness__facts">
              <div>
                <dt>Completed</dt>
                <dd>{formatTime(readiness.backup.completedAt)}</dd>
              </div>
              <div>
                <dt>Driver</dt>
                <dd>{readiness.backup.driver ?? "—"}</dd>
              </div>
            </dl>
          </div>
        </Card>
      </div>

      <Card title={ADMIN_READINESS_UPSTREAMS}>
        {#if readiness.upstreams.length === 0}
          <p class="readiness__empty">{ADMIN_READINESS_NONE}</p>
        {:else}
          <ul class="readiness__upstreams">
            {#each readiness.upstreams as upstream (upstream.upstream)}
              <li>
                <div>
                  <strong>{upstream.upstream}</strong>
                  <span
                    >{upstream.lastOperation ?? "no operation recorded"}</span
                  >
                </div>
                <div class="readiness__upstream-meta">
                  <span>{upstream.consecutiveFailures} failures</span>
                  <Badge
                    label={upstream.state}
                    variant={badgeForUpstream(upstream.state)}
                  />
                </div>
              </li>
            {/each}
          </ul>
        {/if}
      </Card>
    {/if}
  </div>
{/if}

<style>
  .admin {
    display: flex;
    flex: 1;
    flex-direction: column;
    gap: var(--space-6);
    overflow-y: auto;
    padding: var(--space-6);
  }

  .admin--forbidden {
    align-items: center;
    justify-content: center;
  }

  .admin__heading {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-4);
  }

  .admin__title {
    font-size: var(--text-2xl);
  }

  .admin__grid {
    display: grid;
    gap: var(--space-4);
    grid-template-columns: repeat(auto-fit, minmax(18rem, 1fr));
  }

  .readiness__facts {
    display: grid;
    gap: var(--space-2);
  }

  .readiness__facts div {
    display: grid;
    gap: var(--space-2);
    grid-template-columns: minmax(7rem, 0.75fr) minmax(0, 1.25fr);
  }

  .readiness__facts dt,
  .readiness__upstreams span,
  .readiness__empty,
  .readiness__backup p {
    color: var(--muted);
    font-size: var(--text-sm);
  }

  .readiness__facts dd {
    font-family: var(--font-mono);
    overflow-wrap: anywhere;
  }

  .readiness__backup {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .readiness__upstreams {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    list-style: none;
  }

  .readiness__upstreams li,
  .readiness__upstream-meta {
    display: flex;
    align-items: center;
    gap: var(--space-3);
  }

  .readiness__upstreams li {
    justify-content: space-between;
    border: var(--border-width) solid var(--border);
    padding: var(--space-3);
  }

  .readiness__upstreams li > div:first-child {
    display: flex;
    min-width: 0;
    flex-direction: column;
    gap: var(--space-1);
  }

  .readiness__upstream-meta {
    flex-shrink: 0;
  }

  .forbidden {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    text-align: center;
  }

  .forbidden__title {
    color: var(--crit);
    font-size: var(--text-2xl);
  }

  .forbidden__body {
    color: var(--muted);
    font-family: var(--font-mono);
    font-size: var(--text-sm);
  }

  @media (max-width: 40rem) {
    .admin {
      padding: var(--space-4);
    }

    .readiness__upstreams li {
      align-items: flex-start;
      flex-direction: column;
    }
  }
</style>
