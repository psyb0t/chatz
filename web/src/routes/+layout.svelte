<script lang="ts">
  import type { Snippet } from "svelte";
  import "../app.css";
  import "$lib/fonts";
  import { goto } from "$app/navigation";
  import { page } from "$app/state";
  import Sidebar from "$lib/components/Sidebar.svelte";
  import { auth } from "$lib/stores/auth.svelte";
  import { chats } from "$lib/stores/chats.svelte";
  import { models } from "$lib/stores/models.svelte";
  import { mcpServers } from "$lib/stores/mcpServers.svelte";
  import {
    PHASE_ANON,
    PHASE_AUTHED,
    PHASE_LOADING,
    PHASE_SETUP,
  } from "$lib/common/auth";
  import { ROUTE_HOME, ROUTE_LOGIN, ROUTE_SETUP } from "$lib/common/routes";
  import { log } from "$lib/log";
  import { EVENT_APP_BOOT, EVENT_ROUTE_GUARD } from "$lib/common/log-events";
  import {
    TESTID_SIDEBAR_OPEN,
    TESTID_SIDEBAR_BACKDROP,
  } from "$lib/common/test-ids";
  import { A11Y_SIDEBAR_OPEN, A11Y_SIDEBAR_CLOSE } from "$lib/common/labels";
  import { onMount } from "svelte";

  // Phone breakpoint — kept in sync with the `max-width: 640px` media queries in
  // this file and the sidebar's drawer CSS.
  const MOBILE_MEDIA_QUERY = "(max-width: 640px)";

  // Optional UI scale via `?zoom=<n>` (e.g. `?zoom=1.3`) — magnifies the whole
  // app shell while keeping it fit to the viewport (the shell divides its
  // width/height by the same factor, so `zoom` scales it back to exactly one
  // viewport and nothing overflows). Useful for demos, screencasts, and
  // low-vision viewing. Clamped to a sane range.
  const ZOOM_PARAM = "zoom";
  const ZOOM_CSS_VAR = "--zoom";
  const ZOOM_MIN = 0.5;
  const ZOOM_MAX = 3;

  // Optional `?sidebar_collapsed=true` — start with the sidebar collapsed to its
  // rail so the chat pane gets the full width from first paint (screencasts /
  // embeds). Named per-item so future panels can get their own `<item>_collapsed`.
  const SIDEBAR_COLLAPSED_PARAM = "sidebar_collapsed";
  const COLLAPSED_VALUE = "true";

  interface Props {
    children: Snippet;
  }

  const { children }: Props = $props();

  // Left-bar collapse lives here because it also drives the body grid width;
  // the toggle button itself is rendered by the sidebar. This is the DESKTOP
  // affordance (full ↔ rail).
  let sidebarCollapsed = $state(false);

  // Mobile-only state: whether the viewport is a phone, and whether the
  // off-canvas drawer is currently slid open over the content.
  let mobile = $state(false);
  let drawerOpen = $state(false);

  // The sidebar's own toggle button collapses on desktop but closes the drawer
  // on mobile (where the rail concept doesn't apply).
  function toggleSidebar(): void {
    if (mobile) {
      drawerOpen = false;

      return;
    }

    sidebarCollapsed = !sidebarCollapsed;
  }

  function openDrawer(): void {
    drawerOpen = true;
  }

  function closeDrawer(): void {
    drawerOpen = false;
  }

  onMount(() => {
    log.info(EVENT_APP_BOOT);

    const rawZoom = page.url.searchParams.get(ZOOM_PARAM);
    if (rawZoom !== null) {
      const zoom = Number(rawZoom);
      if (Number.isFinite(zoom) && zoom >= ZOOM_MIN && zoom <= ZOOM_MAX) {
        document.documentElement.style.setProperty(ZOOM_CSS_VAR, String(zoom));
      }
    }

    if (
      page.url.searchParams.get(SIDEBAR_COLLAPSED_PARAM) === COLLAPSED_VALUE
    ) {
      sidebarCollapsed = true;
    }

    const mql = window.matchMedia(MOBILE_MEDIA_QUERY);
    mobile = mql.matches;

    const onChange = (event: MediaQueryListEvent): void => {
      mobile = event.matches;
      // Leaving phone width dismisses any open drawer so the desktop grid isn't
      // left with a stray fixed overlay.
      if (!event.matches) {
        drawerOpen = false;
      }
    };

    mql.addEventListener("change", onChange);

    return () => mql.removeEventListener("change", onChange);
  });

  // Navigating (a chat link, New chat, an admin page) closes the drawer, so the
  // freshly-routed content is visible instead of the sidebar staying over it.
  $effect(() => {
    void page.url.pathname;
    drawerOpen = false;
  });

  $effect(() => {
    void auth.refresh();
  });

  // Route the browser to match the auth phase. Setup + anon land on their
  // dedicated pages; an authed user leaving those pages returns home. The app
  // chrome only renders in the authed branch below.
  $effect(() => {
    const path = page.url.pathname;

    if (auth.phase === PHASE_SETUP && path !== ROUTE_SETUP) {
      log.info(EVENT_ROUTE_GUARD, { phase: auth.phase, redirect: ROUTE_SETUP });
      void goto(ROUTE_SETUP);

      return;
    }

    if (auth.phase === PHASE_ANON && path !== ROUTE_LOGIN) {
      log.info(EVENT_ROUTE_GUARD, { phase: auth.phase, redirect: ROUTE_LOGIN });
      void goto(ROUTE_LOGIN);

      return;
    }

    if (
      auth.phase === PHASE_AUTHED &&
      (path === ROUTE_SETUP || path === ROUTE_LOGIN)
    ) {
      log.info(EVENT_ROUTE_GUARD, { phase: auth.phase, redirect: ROUTE_HOME });
      void goto(ROUTE_HOME);
    }
  });

  // Load the authed-only data once, when the session is confirmed. The MCP
  // server list is admin-only server-side (GET /mcp/servers → 403 for a plain
  // user), so it loads ONLY for an admin — a non-admin never triggers the
  // rejected request and never sees the [MCP:n] chip.
  $effect(() => {
    if (auth.phase !== PHASE_AUTHED) {
      return;
    }

    if (!chats.loaded) {
      void chats.load();
    }

    if (!chats.projectsLoaded) {
      void chats.loadProjects();
    }

    if (!models.loaded) {
      void models.load();
    }

    if (auth.user?.isAdmin === true && !mcpServers.loaded) {
      void mcpServers.load();
    }
  });

  const isChrome = $derived(auth.phase === PHASE_AUTHED);
</script>

{#if auth.phase === PHASE_LOADING}
  <div class="boot" id="boot">
    <span class="boot__label">Loading&hellip;</span>
  </div>
{:else if isChrome}
  <div
    class="app"
    class:app--collapsed={sidebarCollapsed}
    class:app--drawer-open={drawerOpen}
    data-testid="app-shell"
  >
    <!-- On mobile the sidebar is always the full drawer (never the rail), so the
         desktop collapse state is suppressed there. -->
    <Sidebar
      collapsed={mobile ? false : sidebarCollapsed}
      onToggle={toggleSidebar}
      {mobile}
      open={drawerOpen}
    />
    <main class="app__main" id="main">
      <!-- Mobile-only affordance to open the off-canvas drawer. -->
      <button
        class="app__hamburger"
        type="button"
        onclick={openDrawer}
        aria-label={A11Y_SIDEBAR_OPEN}
        aria-expanded={drawerOpen}
        data-testid={TESTID_SIDEBAR_OPEN}>&#9776;</button
      >
      {@render children()}
    </main>
    {#if mobile && drawerOpen}
      <!-- Tapping outside the drawer closes it. A button (not a bare div) keeps
           the dismiss target keyboard-focusable and screen-reader-announced. -->
      <button
        class="app__backdrop"
        type="button"
        onclick={closeDrawer}
        aria-label={A11Y_SIDEBAR_CLOSE}
        data-testid={TESTID_SIDEBAR_BACKDROP}
      ></button>
    {/if}
  </div>
{:else}
  {@render children()}
{/if}

<style>
  .app {
    display: grid;
    grid-template-columns: 16rem 1fr;
    /* `--zoom` (set from `?zoom=`) magnifies the whole shell; dividing the
       viewport dimensions by it keeps the zoomed result exactly one viewport,
       so nothing overflows or clips off-screen. Defaults to 1 (no scaling). */
    width: calc(100vw / var(--zoom, 1));
    height: calc(100vh / var(--zoom, 1));
    height: calc(100dvh / var(--zoom, 1));
    zoom: var(--zoom, 1);
    /* The chat pane owns vertical scrolling. Keep oversized generated UI from
       escaping this fixed viewport and creating a second document scrollbar. */
    overflow: hidden;
    /* Animate the column width so collapse/expand glides instead of snapping.
       Interpolating grid-template-columns (rem↔rem, fr↔fr) is supported in all
       current engines. */
    transition: grid-template-columns 0.24s cubic-bezier(0.4, 0, 0.2, 1);
  }

  /* Collapsed: the sidebar shrinks to its slim toggle rail. */
  .app--collapsed {
    grid-template-columns: 3.25rem 1fr;
  }

  @media (prefers-reduced-motion: reduce) {
    .app {
      transition: none;
    }
  }

  .boot {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 100vh;
  }

  .boot__label {
    font-family: var(--font-display);
    font-weight: 500;
    font-size: var(--text-lg);
    color: var(--muted);
  }

  .app__main {
    min-width: 0;
    min-height: 0;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  /* The hamburger + backdrop are mobile-drawer chrome; the desktop grid never
     shows them. */
  .app__hamburger,
  .app__backdrop {
    display: none;
  }

  @media (max-width: 640px) {
    /* The sidebar is a fixed overlay (see Sidebar.svelte), so it's out of the
       grid flow — main gets the whole viewport as a single cell. */
    .app,
    .app--collapsed {
      grid-template-columns: 1fr;
      grid-template-rows: 1fr;
    }

    /* Reserve a band at the top of every authed page for the fixed hamburger so
       it never sits on top of page content. */
    .app__main {
      padding-top: 3rem;
    }

    .app__hamburger {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      position: fixed;
      top: var(--space-2);
      left: var(--space-2);
      z-index: 30;
      line-height: 1;
      padding: var(--space-2) var(--space-3);
      background: var(--panel);
      border: var(--border-width) solid var(--border-strong);
      border-radius: var(--radius);
      color: var(--ink);
      box-shadow: var(--shadow-sm);
    }

    /* Full-viewport dim below the drawer (z 50) and above the hamburger (z 30). */
    .app__backdrop {
      display: block;
      position: fixed;
      inset: 0;
      z-index: 40;
      width: 100%;
      height: 100%;
      border: none;
      border-radius: 0;
      padding: 0;
      background: rgba(0, 0, 0, 0.45);
    }
  }
</style>
