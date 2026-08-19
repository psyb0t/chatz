<script lang="ts">
  import { page } from "$app/state";
  import { goto } from "$app/navigation";
  import { auth } from "$lib/stores/auth.svelte";
  import { chats } from "$lib/stores/chats.svelte";
  import { mcpServers } from "$lib/stores/mcpServers.svelte";
  import {
    theme,
    THEME_TOGGLE_LABEL,
    TESTID_THEME_TOGGLE,
  } from "$lib/stores/theme.svelte";
  import {
    ROUTE_ADMIN_USERS,
    ROUTE_ADMIN_MCP,
    ROUTE_ADMIN_READINESS,
    chatRoute,
  } from "$lib/common/routes";
  import Button from "$lib/components/ui/Button.svelte";
  import StateBlock from "$lib/components/ui/StateBlock.svelte";
  import { STATE_ERROR, STATE_LOADING } from "$lib/components/ui/variants";
  import {
    EMPTY_CHATS,
    STATE_LOADING_CHATS,
    LABEL_LOGOUT,
    SIDEBAR_WORDMARK,
    SIDEBAR_CREDIT_LABEL,
    SIDEBAR_CREDIT_URL,
    A11Y_SIDEBAR_COLLAPSE,
    A11Y_SIDEBAR_EXPAND,
    A11Y_SIDEBAR_CLOSE,
    NAV_ADMIN_USERS,
    NAV_ADMIN_MCP,
    NAV_ADMIN_READINESS,
    A11Y_CHAT_RENAME,
    A11Y_CHAT_SEARCH,
    A11Y_CHAT_DELETE,
    A11Y_CHAT_MENU,
    LABEL_DELETE,
    LABEL_CHAT_EDIT,
    SIDEBAR_SEARCH_PLACEHOLDER,
    mcpChip,
  } from "$lib/common/labels";
  import {
    TESTID_SIDEBAR_LOADING,
    TESTID_SIDEBAR_ERROR,
    TESTID_SIDEBAR_TOGGLE,
    TESTID_NAV_ADMIN_USERS,
    TESTID_NAV_ADMIN_MCP,
    TESTID_NAV_ADMIN_READINESS,
    TESTID_CHAT_RENAME,
    TESTID_CHAT_RENAME_INPUT,
    TESTID_CHAT_DELETE,
    TESTID_CHAT_MENU,
    TESTID_CHAT_SEARCH,
  } from "$lib/common/test-ids";

  interface Props {
    collapsed: boolean;
    onToggle: () => void;
    // Mobile-only: `mobile` flips the layout into the off-canvas drawer mode and
    // `open` drives the slide-in. On desktop both are inert (CSS ignores them).
    mobile: boolean;
    open: boolean;
  }

  const { collapsed, onToggle, mobile, open }: Props = $props();

  let loggingOut = $state(false);

  async function onLogout(): Promise<void> {
    loggingOut = true;
    try {
      await auth.logout();
    } finally {
      loggingOut = false;
    }
  }

  function toUsers(): void {
    void goto(ROUTE_ADMIN_USERS);
  }

  function toMCP(): void {
    void goto(ROUTE_ADMIN_MCP);
  }

  function toReadiness(): void {
    void goto(ROUTE_ADMIN_READINESS);
  }

  function toggleTheme(): void {
    theme.toggle();
  }

  const activeChatId = $derived(page.params.chatId ?? null);
  const isAdmin = $derived(auth.user?.isAdmin === true);
  const mcpCount = $derived(mcpServers.enabledCount);

  // Inline chat rename: the pencil swaps a row's link for an input; Enter (or
  // blur) saves, Escape cancels.
  let renamingId = $state<string | null>(null);
  let renameValue = $state("");
  let renameInputEl: HTMLInputElement | undefined = $state();

  function startRename(event: MouseEvent, id: string, title: string): void {
    event.preventDefault();
    event.stopPropagation();
    renamingId = id;
    renameValue = title;
    queueMicrotask(() => renameInputEl?.focus());
  }

  function cancelRename(): void {
    renamingId = null;
  }

  async function commitRename(): Promise<void> {
    const id = renamingId;
    const title = renameValue.trim();
    renamingId = null;
    if (id === null || title === "") {
      return;
    }

    await chats.rename(id, title);
  }

  function onRenameKeydown(event: KeyboardEvent): void {
    if (event.key === "Enter") {
      event.preventDefault();
      void commitRename();
    } else if (event.key === "Escape") {
      event.preventDefault();
      cancelRename();
    }
  }

  function onSearchInput(event: Event): void {
    const input = event.currentTarget as HTMLInputElement;
    void chats.setSearch(input.value);
  }

  // Per-row action menu: the ⋮ button toggles a small popup holding Edit +
  // Delete, so the row stays a single compact line. Only one is open at a time.
  let menuOpenId = $state<string | null>(null);

  function toggleMenu(event: MouseEvent, id: string): void {
    event.preventDefault();
    menuOpenId = menuOpenId === id ? null : id;
  }

  function closeMenu(): void {
    menuOpenId = null;
  }

  function onMenuEdit(event: MouseEvent, id: string, title: string): void {
    closeMenu();
    startRename(event, id, title);
  }

  function onMenuDelete(id: string): void {
    closeMenu();
    void chats.delete(id);
  }

  // Close the open menu on any pointerdown outside it and on Escape. The
  // actions cluster stops propagation on its own pointerdown (see markup), so
  // interacting with the trigger or the menu never triggers this close.
  $effect(() => {
    if (menuOpenId === null) {
      return;
    }

    const onPointerDown = (): void => closeMenu();
    const onKeyDown = (event: KeyboardEvent): void => {
      if (event.key === "Escape") {
        closeMenu();
      }
    };

    window.addEventListener("pointerdown", onPointerDown);
    window.addEventListener("keydown", onKeyDown);

    return () => {
      window.removeEventListener("pointerdown", onPointerDown);
      window.removeEventListener("keydown", onKeyDown);
    };
  });
</script>

{#if collapsed}
  <aside class="rail" id="sidebar" data-testid="chat-list">
    <button
      class="sidebar__toggle"
      type="button"
      onclick={onToggle}
      aria-label={A11Y_SIDEBAR_EXPAND}
      data-testid={TESTID_SIDEBAR_TOGGLE}
    >
      <!-- Collapsed rail: a hamburger reads as "open the menu" — the standard
           affordance for revealing a hidden panel — rather than a bare chevron. -->
      <svg viewBox="0 0 16 16" aria-hidden="true" focusable="false">
        <path d="M2.5 4.5 H13.5 M2.5 8 H13.5 M2.5 11.5 H13.5" />
      </svg>
    </button>
  </aside>
{:else}
  <aside
    class="sidebar"
    class:sidebar--open={open}
    id="sidebar"
    data-testid="chat-list"
  >
    <div class="sidebar__inner">
      <div class="sidebar__brand">
        <div class="sidebar__brand-text">
          <span class="sidebar__wordmark">{SIDEBAR_WORDMARK}</span>
          <a
            class="sidebar__credit"
            href={SIDEBAR_CREDIT_URL}
            target="_blank"
            rel="noopener noreferrer">{SIDEBAR_CREDIT_LABEL}</a
          >
        </div>
        <!-- Desktop: collapse to the rail. Mobile: close the drawer. -->
        <button
          class="sidebar__toggle"
          type="button"
          onclick={onToggle}
          aria-label={mobile ? A11Y_SIDEBAR_CLOSE : A11Y_SIDEBAR_COLLAPSE}
          data-testid={TESTID_SIDEBAR_TOGGLE}
        >
          <svg viewBox="0 0 16 16" aria-hidden="true" focusable="false">
            {#if mobile}
              <path d="M4 4 12 12M12 4 4 12" />
            {:else}
              <path d="M10 4 6 8 10 12" />
            {/if}
          </svg>
        </button>
      </div>

      <div class="sidebar__section">
        <button
          class="sidebar__new"
          type="button"
          onclick={() => chats.goToNewChat()}>+ New chat</button
        >

        <div class="sidebar__filters">
          <input
            class="sidebar__search"
            value={chats.search}
            oninput={onSearchInput}
            placeholder={SIDEBAR_SEARCH_PLACEHOLDER}
            aria-label={A11Y_CHAT_SEARCH}
            data-testid={TESTID_CHAT_SEARCH}
          />
        </div>

        {#if chats.loading && chats.list.length === 0}
          <div class="sidebar__state">
            <StateBlock
              variant={STATE_LOADING}
              label={STATE_LOADING_CHATS}
              testid={TESTID_SIDEBAR_LOADING}
            />
          </div>
        {:else if chats.error !== null && chats.list.length === 0}
          <div class="sidebar__state">
            <StateBlock
              variant={STATE_ERROR}
              label={chats.error}
              testid={TESTID_SIDEBAR_ERROR}
            />
          </div>
        {:else if chats.list.length === 0}
          <p class="sidebar__empty">{EMPTY_CHATS}</p>
        {:else}
          <ul class="sidebar__list">
            {#each chats.list as chat (chat.id)}
              {@const active = chat.id === activeChatId}
              <li class="sidebar__row">
                {#if renamingId === chat.id}
                  <input
                    class="sidebar__rename-input"
                    bind:this={renameInputEl}
                    bind:value={renameValue}
                    onkeydown={onRenameKeydown}
                    onblur={commitRename}
                    aria-label={A11Y_CHAT_RENAME}
                    data-testid={TESTID_CHAT_RENAME_INPUT}
                  />
                {:else}
                  <div class="sidebar__row-main">
                    <a
                      class="sidebar__item"
                      class:sidebar__item--active={active}
                      href={chatRoute(chat.id)}
                      aria-current={active ? "page" : undefined}
                    >
                      <span class="sidebar__item-name">{chat.title}</span>
                    </a>
                    <!-- The ⋮ trigger + its popup. The trigger and the menu stop
                         their own pointerdown so the window outside-close handler
                         never fires for clicks on them. -->
                    <div
                      class="sidebar__actions"
                      class:sidebar__actions--open={menuOpenId === chat.id}
                    >
                      <button
                        class="icon-btn sidebar__menu-btn"
                        type="button"
                        aria-haspopup="menu"
                        aria-expanded={menuOpenId === chat.id}
                        aria-label={A11Y_CHAT_MENU}
                        data-testid={TESTID_CHAT_MENU}
                        onpointerdown={(e) => e.stopPropagation()}
                        onclick={(e) => toggleMenu(e, chat.id)}>&#8942;</button
                      >
                      {#if menuOpenId === chat.id}
                        <div
                          class="sidebar__menu"
                          role="menu"
                          tabindex="-1"
                          onpointerdown={(e) => e.stopPropagation()}
                        >
                          <button
                            class="sidebar__menu-item"
                            type="button"
                            role="menuitem"
                            onclick={(e) => onMenuEdit(e, chat.id, chat.title)}
                            aria-label={A11Y_CHAT_RENAME}
                            data-testid={TESTID_CHAT_RENAME}
                            >{LABEL_CHAT_EDIT}</button
                          >
                          <button
                            class="sidebar__menu-item sidebar__menu-item--danger"
                            type="button"
                            role="menuitem"
                            onclick={() => onMenuDelete(chat.id)}
                            aria-label={A11Y_CHAT_DELETE}
                            data-testid={TESTID_CHAT_DELETE}
                            >{LABEL_DELETE}</button
                          >
                        </div>
                      {/if}
                    </div>
                  </div>
                {/if}
              </li>
            {/each}
          </ul>
        {/if}
      </div>

      <div class="sidebar__foot">
        {#if isAdmin}
          <nav class="sidebar__nav" aria-label="Admin">
            <Button testid={TESTID_NAV_ADMIN_USERS} onclick={toUsers}>
              {NAV_ADMIN_USERS}
            </Button>
            <Button testid={TESTID_NAV_ADMIN_MCP} onclick={toMCP}>
              {mcpCount > 0
                ? `${NAV_ADMIN_MCP} ${mcpChip(mcpCount)}`
                : NAV_ADMIN_MCP}
            </Button>
            <Button testid={TESTID_NAV_ADMIN_READINESS} onclick={toReadiness}>
              {NAV_ADMIN_READINESS}
            </Button>
          </nav>
        {/if}

        <div class="sidebar__user" id="sidebar-user">
          <span class="sidebar__user-name">{auth.user?.username ?? ""}</span>
          <div class="sidebar__user-actions">
            <button
              class="icon-btn"
              type="button"
              onclick={toggleTheme}
              aria-label={THEME_TOGGLE_LABEL}
              data-testid={TESTID_THEME_TOGGLE}
            >
              {theme.isDark ? "☀" : "☾"}
            </button>
            <Button
              type="button"
              onclick={onLogout}
              disabled={loggingOut}
              ariaLabel={LABEL_LOGOUT}
            >
              {loggingOut ? "…" : LABEL_LOGOUT}
            </Button>
          </div>
        </div>
      </div>
    </div>
  </aside>
{/if}

<style>
  .sidebar {
    display: flex;
    flex-direction: column;
    border-right: var(--border-width) solid var(--border);
    background: var(--panel);
    min-height: 100%;
    max-height: 100%;
    /* Clip the fixed-width inner so the collapse/expand width animation reveals
       it cleanly instead of reflowing (and briefly wrapping) the text. */
    overflow: hidden;
  }

  /* The content keeps its full layout width while the grid column animates in
     and out — the parent just clips it, so nothing reflows mid-animation. */
  .sidebar__inner {
    display: flex;
    flex-direction: column;
    flex: 1;
    min-height: 0;
    width: 100%;
    max-width: 100%;
  }

  /* Collapsed: a slim rail whose only control is the expand toggle, pinned near
     the top so it lines up with the expanded brand row's toggle. It fades in so
     the collapse reads as a transition rather than a hard swap. */
  .rail {
    display: flex;
    flex-direction: column;
    align-items: center;
    border-right: var(--border-width) solid var(--border);
    background: var(--panel);
    min-height: 100%;
    padding: var(--space-4) 0;
    animation: rail-in 0.24s ease both;
  }

  @keyframes rail-in {
    from {
      opacity: 0;
    }

    to {
      opacity: 1;
    }
  }

  /* Clean, consistent collapse/expand control: a rounded chevron button that
     reads the same in the rail and the brand row, with a border that firms up
     on hover instead of the old bare glyph. */
  .sidebar__toggle {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 1.9rem;
    height: 1.9rem;
    flex-shrink: 0;
    /* Icon button: kill the global `button` padding (8px 16px) — on a 1.9rem box
       it consumes the whole width and flex-shrinks the icon to 0, leaving an
       empty rounded rectangle. The fixed-size glyph centers on its own. */
    padding: 0;
    background: transparent;
    border: var(--border-width) solid transparent;
    border-radius: var(--radius);
    color: var(--muted);
    cursor: pointer;
    transition:
      background-color 0.15s ease,
      color 0.15s ease,
      border-color 0.15s ease;
  }

  .sidebar__toggle:hover {
    background: var(--panel-2);
    color: var(--ink);
    border-color: var(--border);
  }

  /* In the collapsed rail the toggle is the ONLY control, so give it a resting
     border + brighter icon — otherwise the rail just looks like empty space. */
  .rail .sidebar__toggle {
    border-color: var(--border-strong);
    color: var(--ink);
  }

  .sidebar__toggle svg {
    width: 1rem;
    height: 1rem;
    flex-shrink: 0;
    fill: none;
    stroke: currentColor;
    stroke-width: 1.75;
    stroke-linecap: round;
    stroke-linejoin: round;
  }

  @media (prefers-reduced-motion: reduce) {
    .rail {
      animation: none;
    }
  }

  .icon-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: var(--border-width) solid transparent;
    border-radius: var(--radius);
    color: var(--muted);
    cursor: pointer;
    line-height: 1;
    padding: var(--space-1) var(--space-2);
    transition:
      background-color 0.12s ease,
      color 0.12s ease;
  }

  .icon-btn:hover {
    background: var(--panel-2);
    color: var(--ink);
  }

  .sidebar__brand {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
    padding: var(--space-4);
  }

  .sidebar__brand-text {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }

  .sidebar__wordmark {
    font-family: var(--font-display);
    font-weight: 600;
    font-size: var(--text-lg);
    letter-spacing: -0.01em;
  }

  .sidebar__credit {
    font-family: var(--font-display);
    font-size: var(--text-xs);
    color: var(--muted);
    text-decoration: none;
  }

  .sidebar__credit:hover {
    color: var(--accent);
    text-decoration: underline;
  }

  /* Chat list is the flexible middle; it scrolls while brand + foot stay put. */
  .sidebar__section {
    flex: 1;
    min-height: 0;
    overflow-x: hidden;
    overflow-y: auto;
    padding: 0 var(--space-3) var(--space-3);
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .sidebar__state {
    margin-top: var(--space-2);
  }

  .sidebar__empty {
    font-family: var(--font-display);
    font-size: var(--text-sm);
    color: var(--muted);
    padding: var(--space-4) var(--space-2);
    text-align: center;
  }

  .sidebar__list {
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .sidebar__item {
    display: block;
    padding: var(--space-2) var(--space-3);
    border-radius: var(--radius);
    text-decoration: none;
    color: var(--muted);
    font-family: var(--font-display);
    font-size: var(--text-sm);
    transition:
      background-color 0.12s ease,
      color 0.12s ease;
  }

  .sidebar__item:hover {
    background: var(--panel-2);
    color: var(--ink);
    text-decoration: none;
  }

  .sidebar__item--active {
    background: var(--accent-soft);
    color: var(--accent);
    font-weight: 600;
  }

  .sidebar__item-name {
    display: block;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  /* One compact line per chat; position:relative anchors the popup menu so it
     never adds height to the row. */
  .sidebar__row {
    position: relative;
  }

  .sidebar__row-main {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    width: 100%;
  }

  .sidebar__row-main .sidebar__item {
    flex: 1;
    min-width: 0;
  }

  /* The ⋮ trigger stays hidden until the row is hovered or focused, or its menu
     is open. focus-within keeps it reachable by keyboard, where there is no
     hover. position:relative anchors the popup to the trigger. */
  .sidebar__actions {
    position: relative;
    flex-shrink: 0;
    opacity: 0;
  }

  .sidebar__row:hover .sidebar__actions,
  .sidebar__row:focus-within .sidebar__actions,
  .sidebar__actions--open {
    opacity: 1;
  }

  .sidebar__menu-btn {
    font-size: var(--text-lg);
    line-height: 1;
  }

  /* Popup anchored under the trigger, absolutely positioned so it overlays the
     list instead of pushing rows apart. */
  .sidebar__menu {
    position: absolute;
    top: calc(100% + 2px);
    right: 0;
    z-index: 20;
    display: flex;
    flex-direction: column;
    min-width: 8rem;
    padding: var(--space-1);
    background: var(--panel);
    border: var(--border-width) solid var(--border-strong);
    border-radius: var(--radius);
    box-shadow: var(--shadow-lg);
  }

  .sidebar__menu-item {
    display: block;
    width: 100%;
    text-align: left;
    padding: var(--space-2) var(--space-3);
    background: transparent;
    border: none;
    border-radius: var(--radius);
    color: var(--ink);
    font-family: var(--font-display);
    font-size: var(--text-sm);
    cursor: pointer;
  }

  .sidebar__menu-item:hover {
    background: var(--panel-2);
  }

  .sidebar__menu-item--danger {
    color: var(--danger, var(--accent));
  }

  .sidebar__menu-item--danger:hover {
    background: var(--danger-soft, var(--accent-soft));
    color: var(--danger, var(--accent));
  }

  .sidebar__rename-input {
    flex: 1;
    min-width: 0;
    font-family: var(--font-display);
    font-size: var(--text-sm);
    padding: var(--space-2) var(--space-3);
    border-radius: var(--radius);
  }

  .sidebar__new {
    display: block;
    text-align: center;
    text-decoration: none;
    color: var(--ink);
    background: var(--bg);
    border: var(--border-width) solid var(--border-strong);
    border-radius: var(--radius);
    padding: var(--space-2) var(--space-4);
    font-family: var(--font-display);
    font-weight: 500;
    font-size: var(--text-sm);
    transition:
      background-color 0.12s ease,
      border-color 0.12s ease;
  }

  .sidebar__new:hover {
    background: var(--panel-2);
    text-decoration: none;
  }

  .sidebar__filters {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .sidebar__search {
    box-sizing: border-box;
    width: 100%;
    min-width: 0;
    border: var(--border-width) solid var(--border);
    border-radius: var(--radius);
    background: var(--bg);
    color: var(--ink);
    font-family: var(--font-display);
    font-size: var(--text-xs);
    padding: var(--space-2);
  }

  /* Bottom cluster: admin nav (Users/MCP) above the user + logout row. */
  .sidebar__foot {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    padding: var(--space-3);
    border-top: var(--border-width) solid var(--border);
  }

  .sidebar__nav {
    display: flex;
    gap: var(--space-2);
  }

  .sidebar__nav :global(.btn) {
    flex: 1;
    font-size: var(--text-xs);
  }

  .sidebar__user {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
  }

  .sidebar__user-name {
    font-family: var(--font-display);
    font-weight: 500;
    font-size: var(--text-sm);
    color: var(--muted);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .sidebar__user-actions {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    flex-shrink: 0;
  }

  .sidebar__user-actions :global(.btn) {
    padding: var(--space-1) var(--space-3);
    font-size: var(--text-xs);
  }

  @media (max-width: 640px) {
    /* Off-canvas drawer: the sidebar leaves the layout flow and floats over the
       chat pane. Closed = slid fully off the left edge; `.sidebar--open` slides
       it back in. A layout-level backdrop dims + closes on an outside tap. The
       layout forces the full sidebar (never the rail) on mobile, so `.rail` is
       unreachable here — only `.sidebar` needs the drawer treatment. */
    .sidebar {
      position: fixed;
      top: 0;
      left: 0;
      z-index: 50;
      width: min(20rem, 85vw);
      height: 100vh;
      height: 100dvh;
      max-height: none;
      border-right: var(--border-width) solid var(--border);
      transform: translateX(-100%);
      transition: transform 0.2s ease;
    }

    .sidebar--open {
      transform: translateX(0);
      box-shadow: var(--shadow-lg);
    }
  }
</style>
