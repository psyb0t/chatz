<script lang="ts">
  import { page } from "$app/state";
  import {
    ROUTE_ADMIN_USERS,
    ROUTE_ADMIN_MCP,
    ROUTE_ADMIN_READINESS,
  } from "$lib/common/routes";
  import {
    NAV_ADMIN_SYSTEM,
    NAV_ADMIN_USERS,
    NAV_ADMIN_MCP,
    NAV_ADMIN_READINESS,
  } from "$lib/common/labels";
  import {
    TESTID_NAV_ADMIN_USERS,
    TESTID_NAV_ADMIN_MCP,
    TESTID_NAV_ADMIN_READINESS,
  } from "$lib/common/test-ids";

  interface Props {
    children: import("svelte").Snippet;
  }

  const { children }: Props = $props();

  // One tab per admin page. href drives navigation; the active tab is derived
  // from the current path so a deep link or reload highlights correctly.
  const tabs = [
    {
      href: ROUTE_ADMIN_USERS,
      label: NAV_ADMIN_USERS,
      testid: TESTID_NAV_ADMIN_USERS,
    },
    {
      href: ROUTE_ADMIN_MCP,
      label: NAV_ADMIN_MCP,
      testid: TESTID_NAV_ADMIN_MCP,
    },
    {
      href: ROUTE_ADMIN_READINESS,
      label: NAV_ADMIN_READINESS,
      testid: TESTID_NAV_ADMIN_READINESS,
    },
  ];

  const activePath = $derived(page.url.pathname);
</script>

<section class="admin">
  <nav class="admin__tabs" aria-label={NAV_ADMIN_SYSTEM}>
    {#each tabs as tab (tab.href)}
      <a
        class="admin__tab"
        class:admin__tab--active={activePath === tab.href}
        href={tab.href}
        aria-current={activePath === tab.href ? "page" : undefined}
        data-testid={tab.testid}
      >
        {tab.label}
      </a>
    {/each}
  </nav>

  <div class="admin__panel">
    {@render children()}
  </div>
</section>

<style>
  .admin {
    display: flex;
    flex-direction: column;
    min-height: 0;
    min-width: 0;
  }

  .admin__tabs {
    display: flex;
    gap: var(--space-1);
    padding: var(--space-3) var(--space-4) 0;
    border-bottom: var(--border-width) solid var(--border);
  }

  .admin__tab {
    padding: var(--space-2) var(--space-4);
    border: var(--border-width) solid transparent;
    border-bottom: none;
    border-top-left-radius: var(--radius);
    border-top-right-radius: var(--radius);
    color: var(--muted);
    font-family: var(--font-display);
    font-size: var(--text-sm);
    text-decoration: none;
    /* Pull the active tab's border down over the container border so it reads
       as a connected tab rather than a floating pill. */
    margin-bottom: calc(-1 * var(--border-width));
  }

  .admin__tab:hover {
    color: var(--ink);
    background: var(--panel-2);
  }

  .admin__tab--active {
    color: var(--ink);
    background: var(--panel);
    border-color: var(--border);
    border-bottom-color: var(--panel);
    font-weight: 600;
  }

  .admin__panel {
    min-width: 0;
  }
</style>
