<script lang="ts">
  import { onMount } from "svelte";
  import { ApiError } from "$lib/api/errors";
  import { auth } from "$lib/stores/auth.svelte";
  import { mcpServers } from "$lib/stores/mcpServers.svelte";
  import type {
    CreateMCPServerRequest,
    UpdateMCPServerRequest,
    MCPServer,
  } from "$lib/api/client";
  import Panel from "$lib/components/ui/Panel.svelte";
  import Field from "$lib/components/ui/Field.svelte";
  import Button from "$lib/components/ui/Button.svelte";
  import Badge from "$lib/components/ui/Badge.svelte";
  import StateBlock from "$lib/components/ui/StateBlock.svelte";
  import Modal from "$lib/components/ui/Modal.svelte";
  import {
    BUTTON_PRIMARY,
    BUTTON_DANGER,
    BADGE_OK,
    BADGE_WARN,
    BADGE_CRIT,
    BADGE_NEUTRAL,
    type BadgeVariant,
    STATE_LOADING,
    STATE_EMPTY,
    STATE_ERROR,
  } from "$lib/components/ui/variants";
  import {
    MCP_STATUS_CONNECTED,
    MCP_STATUS_CONNECTING,
    MCP_STATUS_FAILED,
    mcpHealthDetails,
    mcpStatusLabel,
    mcpReasonLabel,
  } from "$lib/common/mcp";
  import {
    ADMIN_MCP_TITLE,
    ADMIN_MCP_CREATE_TITLE,
    ADMIN_MCP_EDIT_TITLE,
    ADMIN_MCP_IMPORT_TITLE,
    ADMIN_FORBIDDEN_TITLE,
    ADMIN_FORBIDDEN_BODY,
    LABEL_ADD,
    LABEL_DELETE,
    LABEL_EDIT,
    LABEL_RECONNECT,
    LABEL_IMPORT,
    LABEL_SAVE,
    LABEL_CANCEL,
    MCP_TOOLS_NONE,
    MCP_TOOLS_NO_PARAMS,
    MCP_TOOLS_REQUIRED,
    STATE_LOADING as COPY_LOADING,
    EMPTY_MCP,
  } from "$lib/common/labels";
  import {
    TESTID_ADMIN_FORBIDDEN,
    TESTID_ADMIN_LOADING,
    TESTID_ADMIN_EMPTY,
    TESTID_ADMIN_ERROR,
    TESTID_MCP_CREATE_SUBMIT,
    TESTID_MCP_EDIT_SUBMIT,
    TESTID_MCP_MODAL,
    TESTID_MCP_TOOLS_TOGGLE,
    TESTID_MCP_TOOLS_PANEL,
    TESTID_MCP_HEALTH,
    TESTID_MCP_CREATE_NAME,
    TESTID_MCP_CREATE_TRANSPORT,
    TESTID_MCP_CREATE_COMMAND,
    TESTID_MCP_CREATE_ARGS,
    TESTID_MCP_CREATE_URL,
    TESTID_MCP_CREATE_HEADERS,
    TESTID_MCP_CREATE_ENV,
    TESTID_MCP_DELETE,
    TESTID_MCP_EDIT,
    TESTID_MCP_RECONNECT,
    TESTID_MCP_ADD_TOGGLE,
    TESTID_MCP_IMPORT_TOGGLE,
    TESTID_MCP_IMPORT_SUBMIT,
    TESTID_MCP_IMPORT_CONTENT,
  } from "$lib/common/test-ids";

  const isAdmin = $derived(auth.user?.isAdmin === true);

  const TRANSPORT_STDIO = "stdio";
  const TRANSPORT_HTTP = "http";
  type Transport = typeof TRANSPORT_STDIO | typeof TRANSPORT_HTTP;

  const MODE_ADD = "add";
  const MODE_EDIT = "edit";
  type FormMode = typeof MODE_ADD | typeof MODE_EDIT | null;

  // Poll the list while any server is still connecting so its badge settles from
  // connecting → connected/failed without a manual reload.
  const POLL_INTERVAL_MS = 2000;

  // expandedTools tracks which server cards have their tools section open;
  // opening lazily loads that server's tools via the store.
  let expandedTools = $state<Record<string, boolean>>({});

  function toggleTools(serverId: string): void {
    const open = expandedTools[serverId] !== true;
    expandedTools = { ...expandedTools, [serverId]: open };

    if (open) {
      void mcpServers.loadTools(serverId);
    }
  }

  // A server's tools cache is invalidated (set to undefined) by the store on
  // update/reconnect/delete so a re-expand refetches fresh data — but a panel
  // that is ALREADY expanded when that happens has no re-expand to trigger a
  // refetch, so it would otherwise sit stuck showing stale "No tools
  // available" for a server that is actually connected with tools. Re-fetch
  // for any still-expanded server whose cache just went missing.
  $effect(() => {
    for (const serverId of Object.keys(expandedTools)) {
      if (
        expandedTools[serverId] === true &&
        mcpServers.tools[serverId] === undefined
      ) {
        void mcpServers.loadTools(serverId);
      }
    }
  });

  interface ParamRow {
    name: string;
    type: string;
    required: boolean;
    description: string;
  }

  // paramEntries flattens a tool's JSON-Schema `parameters` into a render list;
  // a missing / non-object `properties` yields [].
  function paramEntries(parameters: Record<string, unknown>): ParamRow[] {
    const props = parameters.properties;
    if (props === null || typeof props !== "object") {
      return [];
    }

    const required = Array.isArray(parameters.required)
      ? (parameters.required as unknown[]).map(String)
      : [];

    return Object.entries(props as Record<string, unknown>).map(
      ([name, raw]) => {
        const schema =
          raw !== null && typeof raw === "object"
            ? (raw as Record<string, unknown>)
            : {};

        return {
          name,
          type: typeof schema.type === "string" ? schema.type : "",
          required: required.includes(name),
          description:
            typeof schema.description === "string" ? schema.description : "",
        };
      },
    );
  }

  onMount(() => {
    if (isAdmin && !mcpServers.loaded) {
      void mcpServers.load();
    }
  });

  $effect(() => {
    if (!mcpServers.anySettling) {
      return;
    }

    const id = setInterval(() => {
      void mcpServers.refresh();
    }, POLL_INTERVAL_MS);

    return () => clearInterval(id);
  });

  let mode = $state<FormMode>(null);
  let editingId = $state<string | null>(null);
  let name = $state("");
  let transport = $state<Transport>(TRANSPORT_STDIO);
  let command = $state("");
  let argsText = $state("");
  let url = $state("");
  let headersText = $state("");
  let envText = $state("");
  let enabled = $state(true);
  let formError = $state("");
  let submitting = $state(false);

  let showImport = $state(false);
  let importText = $state("");
  let importError = $state("");
  let importing = $state(false);

  let busyId = $state<string | null>(null);

  function parseLines(raw: string): string[] {
    return raw
      .split(/\r?\n|\s+/)
      .map((s) => s.trim())
      .filter((s) => s.length > 0);
  }

  function parseKeyVals(raw: string): Record<string, string> {
    const out: Record<string, string> = {};
    for (const line of raw.split(/\r?\n/)) {
      const trimmed = line.trim();
      if (trimmed.length === 0) {
        continue;
      }
      const eq = trimmed.indexOf("=");
      if (eq <= 0) {
        continue;
      }
      out[trimmed.slice(0, eq).trim()] = trimmed.slice(eq + 1);
    }

    return out;
  }

  function resetForm(): void {
    name = "";
    transport = TRANSPORT_STDIO;
    command = "";
    argsText = "";
    url = "";
    headersText = "";
    envText = "";
    enabled = true;
    formError = "";
  }

  function openAdd(): void {
    resetForm();
    editingId = null;
    showImport = false;
    mode = MODE_ADD;
  }

  function kvToText(map: Record<string, string>): string {
    return Object.entries(map)
      .map(([k, v]) => `${k}=${v}`)
      .join("\n");
  }

  function openEdit(server: MCPServer): void {
    resetForm();
    editingId = server.id;
    name = server.name;
    transport = server.transport;
    command = server.command ?? "";
    url = server.url ?? "";
    enabled = server.enabled;
    // Authorization values arrive masked; resubmitting an unchanged mask keeps
    // the sealed credential, while clearing the header removes it.
    argsText = (server.args ?? []).join("\n");
    envText = kvToText(server.env ?? {});
    headersText = kvToText(server.headers ?? {});
    showImport = false;
    mode = MODE_EDIT;
  }

  function closeForm(): void {
    mode = null;
    editingId = null;
  }

  function toggleImport(): void {
    showImport = !showImport;
    mode = null;
    importError = "";
  }

  // buildTransportFields fills the transport-specific fields shared by create +
  // update. In edit mode (includeEmpty) the maps are ALWAYS sent — the form is
  // prefilled from the server response, so a cleared field means "remove it",
  // not "keep the stored value". In add mode empty maps are omitted.
  function buildTransportFields(
    includeEmpty: boolean,
  ): Partial<CreateMCPServerRequest> {
    const out: Partial<CreateMCPServerRequest> = {};
    if (transport === TRANSPORT_STDIO) {
      out.command = command;
      const args = parseLines(argsText);
      if (includeEmpty || args.length > 0) {
        out.args = args;
      }
      const env = parseKeyVals(envText);
      if (includeEmpty || Object.keys(env).length > 0) {
        out.env = env;
      }

      return out;
    }

    out.url = url;
    const headers = parseKeyVals(headersText);
    if (includeEmpty || Object.keys(headers).length > 0) {
      out.headers = headers;
    }

    return out;
  }

  async function onSubmit(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    formError = "";
    submitting = true;

    try {
      if (mode === MODE_EDIT && editingId !== null) {
        const body: UpdateMCPServerRequest = {
          name,
          transport,
          enabled,
          ...buildTransportFields(true),
        };
        await mcpServers.update(editingId, body);
      } else {
        const body: CreateMCPServerRequest = {
          name,
          transport,
          ...buildTransportFields(false),
        };
        await mcpServers.create(body);
      }
      closeForm();
    } catch (err) {
      formError =
        err instanceof ApiError ? err.message : "save failed, try again";
    } finally {
      submitting = false;
    }
  }

  async function onDelete(serverId: string): Promise<void> {
    busyId = serverId;
    try {
      await mcpServers.remove(serverId);
    } catch (err) {
      formError =
        err instanceof ApiError ? err.message : "delete failed, try again";
    } finally {
      busyId = null;
    }
  }

  async function onReconnect(serverId: string): Promise<void> {
    busyId = serverId;
    try {
      await mcpServers.reconnect(serverId);
    } catch (err) {
      formError =
        err instanceof ApiError ? err.message : "reconnect failed, try again";
    } finally {
      busyId = null;
    }
  }

  async function onImport(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    importError = "";
    importing = true;

    try {
      await mcpServers.importJSON(importText);
      importText = "";
      showImport = false;
    } catch (err) {
      importError =
        err instanceof ApiError ? err.message : "import failed, try again";
    } finally {
      importing = false;
    }
  }

  function badgeVariant(status: string): BadgeVariant {
    switch (status) {
      case MCP_STATUS_CONNECTED:
        return BADGE_OK;
      case MCP_STATUS_CONNECTING:
        return BADGE_WARN;
      case MCP_STATUS_FAILED:
        return BADGE_CRIT;
      default:
        return BADGE_NEUTRAL;
    }
  }

  function statusDetail(server: MCPServer): string | undefined {
    if (server.status !== MCP_STATUS_FAILED) {
      return undefined;
    }

    const reason = mcpReasonLabel(server.statusReason ?? "");

    return server.statusError ? `${reason}: ${server.statusError}` : reason;
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
    <div class="admin__head">
      <h1 class="admin__title">{ADMIN_MCP_TITLE}</h1>
      <div class="admin__head-actions">
        <Button testid={TESTID_MCP_ADD_TOGGLE} onclick={openAdd}>
          {ADMIN_MCP_CREATE_TITLE}
        </Button>
        <Button testid={TESTID_MCP_IMPORT_TOGGLE} onclick={toggleImport}>
          {ADMIN_MCP_IMPORT_TITLE}
        </Button>
      </div>
    </div>

    {#if mode !== null}
      <Modal
        title={mode === MODE_EDIT
          ? ADMIN_MCP_EDIT_TITLE
          : ADMIN_MCP_CREATE_TITLE}
        onClose={closeForm}
        testid={TESTID_MCP_MODAL}
      >
        <form class="admin__form" onsubmit={onSubmit}>
          <Field label="Name">
            {#snippet control()}
              <input
                class="admin__input"
                type="text"
                autocomplete="off"
                data-testid={TESTID_MCP_CREATE_NAME}
                bind:value={name}
                required
              />
            {/snippet}
          </Field>

          <Field label="Transport">
            {#snippet control()}
              <select
                class="admin__input"
                data-testid={TESTID_MCP_CREATE_TRANSPORT}
                bind:value={transport}
              >
                <option value={TRANSPORT_STDIO}>stdio</option>
                <option value={TRANSPORT_HTTP}>http</option>
              </select>
            {/snippet}
          </Field>

          {#if transport === TRANSPORT_STDIO}
            <Field label="Command">
              {#snippet control()}
                <input
                  class="admin__input"
                  type="text"
                  autocomplete="off"
                  data-testid={TESTID_MCP_CREATE_COMMAND}
                  bind:value={command}
                  required
                />
              {/snippet}
            </Field>

            <Field label="Args (one per line)">
              {#snippet control()}
                <textarea
                  class="admin__input admin__textarea"
                  data-testid={TESTID_MCP_CREATE_ARGS}
                  bind:value={argsText}
                ></textarea>
              {/snippet}
            </Field>

            <Field label="Env (KEY=value per line)">
              {#snippet control()}
                <textarea
                  class="admin__input admin__textarea admin__textarea--tall"
                  data-testid={TESTID_MCP_CREATE_ENV}
                  bind:value={envText}
                ></textarea>
              {/snippet}
            </Field>
          {:else}
            <Field label="URL">
              {#snippet control()}
                <input
                  class="admin__input"
                  type="url"
                  autocomplete="off"
                  data-testid={TESTID_MCP_CREATE_URL}
                  bind:value={url}
                  required
                />
              {/snippet}
            </Field>

            <Field label="Headers (KEY=value per line)">
              {#snippet control()}
                <textarea
                  class="admin__input admin__textarea admin__textarea--tall"
                  data-testid={TESTID_MCP_CREATE_HEADERS}
                  bind:value={headersText}
                ></textarea>
              {/snippet}
            </Field>
          {/if}

          {#if mode === MODE_EDIT}
            <label class="admin__check">
              <input type="checkbox" bind:checked={enabled} />
              <span>Enabled</span>
            </label>
          {/if}

          {#if formError}
            <span class="admin__error" role="alert">{formError}</span>
          {/if}

          <div class="admin__form-actions">
            <Button type="button" onclick={closeForm}>{LABEL_CANCEL}</Button>
            <Button
              variant={BUTTON_PRIMARY}
              type="submit"
              testid={mode === MODE_EDIT
                ? TESTID_MCP_EDIT_SUBMIT
                : TESTID_MCP_CREATE_SUBMIT}
              disabled={submitting}
            >
              {submitting
                ? "Saving…"
                : mode === MODE_EDIT
                  ? LABEL_SAVE
                  : LABEL_ADD}
            </Button>
          </div>
        </form>
      </Modal>
    {/if}

    {#if showImport}
      <Modal
        title={ADMIN_MCP_IMPORT_TITLE}
        onClose={toggleImport}
        testid={TESTID_MCP_MODAL}
      >
        <form class="admin__form" onsubmit={onImport}>
          <Field label=".mcp.json" error={importError || null}>
            {#snippet control()}
              <textarea
                class="admin__input admin__textarea admin__textarea--tall"
                data-testid={TESTID_MCP_IMPORT_CONTENT}
                bind:value={importText}
                required
              ></textarea>
            {/snippet}
          </Field>

          <div class="admin__form-actions">
            <Button type="button" onclick={toggleImport}>{LABEL_CANCEL}</Button>
            <Button
              variant={BUTTON_PRIMARY}
              type="submit"
              testid={TESTID_MCP_IMPORT_SUBMIT}
              disabled={importing}
            >
              {importing ? "Importing…" : LABEL_IMPORT}
            </Button>
          </div>
        </form>
      </Modal>
    {/if}

    {#if mcpServers.loading && mcpServers.list.length === 0}
      <StateBlock
        variant={STATE_LOADING}
        label={COPY_LOADING}
        testid={TESTID_ADMIN_LOADING}
      />
    {:else if mcpServers.error !== null && mcpServers.list.length === 0}
      <StateBlock
        variant={STATE_ERROR}
        label={mcpServers.error}
        testid={TESTID_ADMIN_ERROR}
      />
    {:else if mcpServers.list.length === 0}
      <StateBlock
        variant={STATE_EMPTY}
        label={EMPTY_MCP}
        testid={TESTID_ADMIN_EMPTY}
      />
    {:else}
      <ul class="mcp-list">
        {#each mcpServers.list as s (s.id)}
          {@const healthDetails = mcpHealthDetails(s)}
          <li class="mcp-row">
            <div class="mcp-row__main">
              <span class="mcp-row__name">{s.name}</span>
              <span class="mcp-row__meta">{s.transport}</span>
            </div>

            <div class="mcp-row__status" title={statusDetail(s)}>
              <Badge
                label={mcpStatusLabel(s.status)}
                variant={badgeVariant(s.status)}
              />
              {#if s.status === MCP_STATUS_CONNECTED && s.toolCount != null && s.toolCount > 0}
                <button
                  class="mcp-row__tools-toggle"
                  type="button"
                  aria-expanded={expandedTools[s.id] === true}
                  data-testid={TESTID_MCP_TOOLS_TOGGLE}
                  onclick={() => toggleTools(s.id)}
                >
                  {s.toolCount} tools {expandedTools[s.id] ? "▾" : "▸"}
                </button>
              {:else if s.toolCount != null}
                <span class="mcp-row__tools">{s.toolCount} tools</span>
              {/if}
            </div>

            {#if healthDetails.length > 0}
              <div class="mcp-row__health" data-testid={TESTID_MCP_HEALTH}>
                {#each healthDetails as detail}
                  <span>{detail}</span>
                {/each}
              </div>
            {/if}

            <div class="mcp-row__actions">
              <Button
                testid={TESTID_MCP_RECONNECT}
                disabled={busyId === s.id}
                onclick={() => onReconnect(s.id)}
              >
                {LABEL_RECONNECT}
              </Button>
              <Button
                testid={TESTID_MCP_EDIT}
                disabled={busyId === s.id}
                onclick={() => openEdit(s)}
              >
                {LABEL_EDIT}
              </Button>
              <Button
                variant={BUTTON_DANGER}
                testid={TESTID_MCP_DELETE}
                disabled={busyId === s.id}
                onclick={() => onDelete(s.id)}
              >
                {LABEL_DELETE}
              </Button>
            </div>

            {#if expandedTools[s.id]}
              {@const ts = mcpServers.tools[s.id]}
              <div class="mcp-tools" data-testid={TESTID_MCP_TOOLS_PANEL}>
                {#if ts?.loading}
                  <p class="mcp-tools__note">{COPY_LOADING}</p>
                {:else if ts?.error != null}
                  <p class="mcp-tools__error">{ts.error}</p>
                {:else if ts?.list != null && ts.list.length > 0}
                  <ul class="mcp-tools__list">
                    {#each ts.list as tool (tool.name)}
                      {@const params = paramEntries(tool.parameters)}
                      <li class="mcp-tool">
                        <span class="mcp-tool__name">{tool.name}</span>
                        {#if tool.description}
                          <p class="mcp-tool__desc">{tool.description}</p>
                        {/if}
                        {#if params.length > 0}
                          <ul class="mcp-tool__params">
                            {#each params as p (p.name)}
                              <li class="mcp-param">
                                <code class="mcp-param__name">{p.name}</code>
                                {#if p.type}
                                  <span class="mcp-param__type">{p.type}</span>
                                {/if}
                                {#if p.required}
                                  <span class="mcp-param__req">
                                    {MCP_TOOLS_REQUIRED}
                                  </span>
                                {/if}
                                {#if p.description}
                                  <span class="mcp-param__desc">
                                    {p.description}
                                  </span>
                                {/if}
                              </li>
                            {/each}
                          </ul>
                        {:else}
                          <p class="mcp-tool__noparams">
                            {MCP_TOOLS_NO_PARAMS}
                          </p>
                        {/if}
                      </li>
                    {/each}
                  </ul>
                {:else}
                  <p class="mcp-tools__note">{MCP_TOOLS_NONE}</p>
                {/if}
              </div>
            {/if}
          </li>
        {/each}
      </ul>
    {/if}
  </div>
{/if}

<style>
  .admin {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    padding: var(--space-6);
    overflow-y: auto;
  }

  .admin--forbidden {
    align-items: center;
    justify-content: center;
    flex: 1;
  }

  .admin__head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    flex-wrap: wrap;
  }

  .admin__head-actions {
    display: flex;
    gap: var(--space-2);
  }

  .admin__title {
    font-size: var(--text-2xl);
  }

  .admin__form {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .admin__form-actions {
    display: flex;
    justify-content: flex-end;
    gap: var(--space-2);
  }

  .admin__input {
    width: 100%;
    font-family: var(--font-mono);
    font-size: var(--text-sm);
  }

  .admin__textarea {
    min-height: 4rem;
    resize: vertical;
  }

  .admin__textarea--tall {
    min-height: 8rem;
  }

  .admin__check {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-size: var(--text-sm);
  }

  .admin__error {
    border: var(--border-width) solid var(--crit);
    border-radius: var(--radius);
    color: var(--crit);
    background: var(--bg);
    padding: var(--space-2) var(--space-3);
    font-size: var(--text-sm);
  }

  .mcp-list {
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .mcp-row {
    display: flex;
    align-items: center;
    gap: var(--space-4);
    flex-wrap: wrap;
    border: var(--border-width) solid var(--border);
    border-radius: var(--radius);
    background: var(--panel);
    box-shadow: var(--shadow-sm);
    padding: var(--space-3) var(--space-4);
  }

  .mcp-row__main {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
    flex: 1;
  }

  .mcp-row__name {
    font-weight: 600;
    font-size: var(--text-sm);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .mcp-row__meta {
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    color: var(--muted);
  }

  .mcp-row__status {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }

  .mcp-row__tools {
    font-size: var(--text-xs);
    color: var(--muted);
  }

  .mcp-row__health {
    flex-basis: 100%;
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-1) var(--space-3);
    color: var(--muted);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    overflow-wrap: anywhere;
  }

  .mcp-row__actions {
    display: flex;
    gap: var(--space-2);
  }

  .mcp-row__actions :global(.btn) {
    font-size: var(--text-xs);
  }

  .mcp-row__tools-toggle {
    background: transparent;
    border: var(--border-width) solid transparent;
    border-radius: var(--radius-sm);
    color: var(--muted);
    cursor: pointer;
    font-size: var(--text-xs);
    padding: 2px var(--space-1);
  }

  .mcp-row__tools-toggle:hover {
    background: var(--panel-2);
    color: var(--ink);
  }

  /* Full-width disclosure: flex-basis 100% wraps it below the row (the card is
     a wrapping flex container). */
  .mcp-tools {
    flex-basis: 100%;
    border-top: var(--border-width) solid var(--border);
    padding-top: var(--space-3);
    margin-top: var(--space-1);
  }

  .mcp-tools__note {
    font-size: var(--text-xs);
    color: var(--muted);
  }

  .mcp-tools__error {
    font-size: var(--text-xs);
    color: var(--crit);
  }

  .mcp-tools__list {
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .mcp-tool {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .mcp-tool__name {
    font-family: var(--font-mono);
    font-size: var(--text-sm);
    font-weight: 600;
  }

  .mcp-tool__desc {
    font-size: var(--text-xs);
    color: var(--muted);
  }

  .mcp-tool__noparams {
    font-size: var(--text-xs);
    color: var(--faint);
  }

  .mcp-tool__params {
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: 2px;
    margin-top: 2px;
    padding-left: var(--space-3);
    border-left: var(--border-width) solid var(--border);
  }

  .mcp-param {
    display: flex;
    align-items: baseline;
    flex-wrap: wrap;
    gap: var(--space-2);
    font-size: var(--text-xs);
  }

  .mcp-param__name {
    font-family: var(--font-mono);
    color: var(--ink);
  }

  .mcp-param__type {
    font-family: var(--font-mono);
    color: var(--accent);
  }

  .mcp-param__req {
    color: var(--crit);
    font-weight: 600;
  }

  .mcp-param__desc {
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
    font-size: var(--text-sm);
  }
</style>
