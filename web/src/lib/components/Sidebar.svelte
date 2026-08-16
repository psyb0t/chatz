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
    A11Y_CHAT_ARCHIVE,
    A11Y_CHAT_PROJECT_ASSIGNMENT,
    A11Y_CHAT_PROJECT_FILTER,
    A11Y_CHAT_PIN,
    A11Y_CHAT_SEARCH,
    A11Y_CHAT_UNARCHIVE,
    A11Y_CHAT_UNPIN,
    LABEL_ARCHIVE,
    LABEL_DELETE,
    LABEL_PIN,
    LABEL_UNARCHIVE,
    LABEL_UNPIN,
    SIDEBAR_ACTIVE_CHATS,
    SIDEBAR_ALL_PROJECTS,
    SIDEBAR_ARCHIVED_CHATS,
    SIDEBAR_NEW_PROJECT_PLACEHOLDER,
    SIDEBAR_NO_PROJECT,
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
    TESTID_CHAT_ARCHIVE,
    TESTID_CHAT_ARCHIVE_TOGGLE,
    TESTID_CHAT_DELETE,
    TESTID_CHAT_PIN,
    TESTID_CHAT_PROJECT_ASSIGNMENT,
    TESTID_CHAT_PROJECT_CREATE,
    TESTID_CHAT_PROJECT_CREATE_INPUT,
    TESTID_CHAT_PROJECT_FILTER,
    TESTID_CHAT_SEARCH,
  } from "$lib/common/test-ids";

  const EMPTY_PROJECT_ID = "";

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
  let newProjectName = $state("");

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

  function onProjectFilterChange(event: Event): void {
    const select = event.currentTarget as HTMLSelectElement;
    void chats.setProject(
      select.value === EMPTY_PROJECT_ID ? null : select.value,
    );
  }

  function onChatProjectChange(event: Event, chatId: string): void {
    const select = event.currentTarget as HTMLSelectElement;
    void chats.setChatProject(
      chatId,
      select.value === EMPTY_PROJECT_ID ? null : select.value,
    );
  }

  async function createProject(): Promise<void> {
    const name = newProjectName.trim();
    if (name === EMPTY_PROJECT_ID) {
      return;
    }

    await chats.createProject(name);
    newProjectName = EMPTY_PROJECT_ID;
  }

  function onNewProjectKeydown(event: KeyboardEvent): void {
    if (event.key !== "Enter") {
      return;
    }

    event.preventDefault();
    void createProject();
  }

  function projectName(projectId: string | undefined): string {
    return (
      chats.projects.find((project) => project.id === projectId)?.name ??
      SIDEBAR_NO_PROJECT
    );
  }
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
          <div
            class="sidebar__view-toggle"
            data-testid={TESTID_CHAT_ARCHIVE_TOGGLE}
          >
            <button
              class:sidebar__view-button--active={!chats.archived}
              class="sidebar__view-button"
              type="button"
              onclick={() => chats.showArchived(false)}
              >{SIDEBAR_ACTIVE_CHATS}</button
            >
            <button
              class:sidebar__view-button--active={chats.archived}
              class="sidebar__view-button"
              type="button"
              onclick={() => chats.showArchived(true)}
              >{SIDEBAR_ARCHIVED_CHATS}</button
            >
          </div>
          <select
            class="sidebar__project-filter"
            value={chats.projectId ?? EMPTY_PROJECT_ID}
            onchange={onProjectFilterChange}
            aria-label={A11Y_CHAT_PROJECT_FILTER}
            data-testid={TESTID_CHAT_PROJECT_FILTER}
          >
            <option value={EMPTY_PROJECT_ID}>{SIDEBAR_ALL_PROJECTS}</option>
            {#each chats.projects as project (project.id)}
              <option value={project.id}>{project.name}</option>
            {/each}
          </select>
          <div class="sidebar__project-create">
            <input
              bind:value={newProjectName}
              onkeydown={onNewProjectKeydown}
              placeholder={SIDEBAR_NEW_PROJECT_PLACEHOLDER}
              data-testid={TESTID_CHAT_PROJECT_CREATE_INPUT}
            />
            <button
              type="button"
              onclick={createProject}
              data-testid={TESTID_CHAT_PROJECT_CREATE}>+</button
            >
          </div>
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
                      {#if chat.projectId !== undefined}
                        <span class="sidebar__project-name"
                          >{projectName(chat.projectId)}</span
                        >
                      {/if}
                    </a>
                  </div>
                  <div class="sidebar__actions">
                    <button
                      class="icon-btn sidebar__rename"
                      type="button"
                      onclick={(e) => startRename(e, chat.id, chat.title)}
                      aria-label={A11Y_CHAT_RENAME}
                      data-testid={TESTID_CHAT_RENAME}>&#9998;</button
                    >
                    <button
                      class="icon-btn"
                      type="button"
                      onclick={() =>
                        chat.pinnedAt === undefined
                          ? chats.pin(chat.id)
                          : chats.unpin(chat.id)}
                      aria-label={chat.pinnedAt === undefined
                        ? A11Y_CHAT_PIN
                        : A11Y_CHAT_UNPIN}
                      data-testid={TESTID_CHAT_PIN}
                      >{chat.pinnedAt === undefined
                        ? LABEL_PIN
                        : LABEL_UNPIN}</button
                    >
                    <button
                      class="icon-btn"
                      type="button"
                      onclick={() =>
                        chat.archivedAt === undefined
                          ? chats.archive(chat.id)
                          : chats.unarchive(chat.id)}
                      aria-label={chat.archivedAt === undefined
                        ? A11Y_CHAT_ARCHIVE
                        : A11Y_CHAT_UNARCHIVE}
                      data-testid={TESTID_CHAT_ARCHIVE}
                      >{chat.archivedAt === undefined
                        ? LABEL_ARCHIVE
                        : LABEL_UNARCHIVE}</button
                    >
                    <button
                      class="icon-btn"
                      type="button"
                      onclick={() => chats.delete(chat.id)}
                      data-testid={TESTID_CHAT_DELETE}>{LABEL_DELETE}</button
                    >
                  </div>
                  <select
                    class="sidebar__chat-project"
                    value={chat.projectId ?? EMPTY_PROJECT_ID}
                    onchange={(event) => onChatProjectChange(event, chat.id)}
                    aria-label={A11Y_CHAT_PROJECT_ASSIGNMENT}
                    data-testid={TESTID_CHAT_PROJECT_ASSIGNMENT}
                  >
                    <option value={EMPTY_PROJECT_ID}
                      >{SIDEBAR_NO_PROJECT}</option
                    >
                    {#each chats.projects as project (project.id)}
                      <option value={project.id}>{project.name}</option>
                    {/each}
                  </select>
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

  .sidebar__row {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
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

  .sidebar__actions {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 2px;
    width: 100%;
  }

  /* Pencil reveals on row hover (or when it takes focus via keyboard). */
  .sidebar__rename {
    flex-shrink: 0;
    opacity: 0;
    font-size: var(--text-xs);
  }

  .sidebar__row:hover .sidebar__rename,
  .sidebar__rename:focus-visible {
    opacity: 1;
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

  .sidebar__search,
  .sidebar__project-filter,
  .sidebar__chat-project,
  .sidebar__project-create input {
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

  .sidebar__view-toggle {
    display: grid;
    grid-template-columns: 1fr 1fr;
    border: var(--border-width) solid var(--border);
    border-radius: var(--radius);
    overflow: hidden;
  }

  .sidebar__view-button {
    border: 0;
    border-radius: 0;
    background: var(--bg);
    color: var(--muted);
    font-family: var(--font-display);
    font-size: var(--text-xs);
    padding: var(--space-2);
  }

  .sidebar__view-button--active {
    background: var(--accent-soft);
    color: var(--accent);
  }

  .sidebar__project-create {
    display: flex;
    gap: var(--space-1);
  }

  .sidebar__project-create button {
    flex-shrink: 0;
    padding: var(--space-1) var(--space-3);
  }

  .sidebar__project-name {
    display: block;
    color: var(--muted);
    font-size: var(--text-xs);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .sidebar__chat-project {
    display: none;
  }

  .sidebar__row:hover .sidebar__chat-project,
  .sidebar__chat-project:focus {
    display: block;
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
