//go:build integration

package httpservertest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/psyb0t/aichteeteapee"
	"github.com/psyb0t/chatz/internal/pkg/config"
	"github.com/psyb0t/chatz/internal/pkg/core/auth"
	"github.com/psyb0t/chatz/internal/pkg/core/chats"
	"github.com/psyb0t/chatz/internal/pkg/db"
	"github.com/psyb0t/chatz/internal/pkg/db/models"
	"github.com/psyb0t/chatz/internal/pkg/db/repositories"
	"github.com/psyb0t/chatz/internal/pkg/http/server"
	"github.com/psyb0t/chatz/internal/pkg/mcp"
	"github.com/psyb0t/chatz/internal/pkg/metrics"
	"github.com/psyb0t/chatz/internal/pkg/operations"
	"github.com/psyb0t/chatz/internal/pkg/secrets"
	"github.com/psyb0t/chatz/internal/pkg/upstreams"
	"github.com/psyb0t/chatz/internal/pkg/usage"
	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/elelem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

const (
	testModel     = "test-model"
	fakeReply     = "hello from fake"
	adminUser     = "admin"
	adminPassword = "pw-abcdef"

	// Repeated route paths.
	pathAuthSetup   = "/api/v1/auth/setup"
	pathAuthStatus  = "/api/v1/auth/status"
	pathAuthLogin   = "/api/v1/auth/login"
	pathModels      = "/api/v1/models"
	pathUpstreams   = "/api/v1/upstreams"
	pathReadiness   = "/api/v1/admin/readiness"
	pathChats       = "/api/v1/chats"
	pathUsers       = "/api/v1/users"
	pathMCPServers  = "/api/v1/mcp/servers"
	pathChatsPrefix = "/api/v1/chats/"

	// Repeated error-code strings not already covered by
	// aichteeteapee.ErrorCode*.
	errCodeValidationFailed = aichteeteapee.ErrorCodeValidationFailed
	errCodeNotFound         = aichteeteapee.ErrorCodeNotFound
	errCodeUnauthorized     = aichteeteapee.ErrorCodeUnauthorized
	errCodeBadRequest       = aichteeteapee.ErrorCodeBadRequest

	rejectAssistantMessageInsert = `
CREATE FUNCTION reject_assistant_message_insert() RETURNS trigger AS $$
BEGIN
    IF NEW.role = 'assistant' THEN
        RAISE EXCEPTION 'reject assistant message insert for persistence test';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER reject_assistant_message_insert_before_insert
BEFORE INSERT ON messages
FOR EACH ROW EXECUTE FUNCTION reject_assistant_message_insert();
`
	removeRejectAssistantMessageInsert = `
DROP TRIGGER reject_assistant_message_insert_before_insert ON messages;
DROP FUNCTION reject_assistant_message_insert();
`
)

// fakeLLM streams a fixed reply, advertises one model, and records requests.
type fakeLLM struct {
	mu       sync.Mutex
	requests []elelem.DriverRequest
	stream   fakeStreamFunc
}

type fakeStreamFunc func(
	ctx context.Context,
	callIndex int,
	req elelem.DriverRequest,
	onDelta func(elelem.Delta) error,
) (elelem.Usage, error)

func (f *fakeLLM) Stream(
	ctx context.Context,
	req elelem.DriverRequest,
	onDelta func(elelem.Delta) error,
) (elelem.Usage, error) {
	req.Messages = append([]elelem.Message(nil), req.Messages...)
	req.Tools = append([]elelem.Tool(nil), req.Tools...)

	f.mu.Lock()
	f.requests = append(f.requests, req)
	callIndex := len(f.requests) - 1
	stream := f.stream
	f.mu.Unlock()

	if stream != nil {
		return stream(ctx, callIndex, req, onDelta)
	}

	if err := onDelta(elelem.Delta{Text: fakeReply}); err != nil {
		return elelem.Usage{}, err
	}

	return elelem.Usage{
		TokenCounts:  elelem.TokenCounts{Completion: 3, Total: 3},
		FinishReason: elelem.FinishReasonStop,
	}, nil
}

// Complete replays through the same body as Stream: this double records what
// the server asked for, and that recording must not depend on which transport
// elelem picked. A separate stub here would let a non-streaming run pass while
// recording nothing.
func (f *fakeLLM) Complete(
	ctx context.Context,
	req elelem.DriverRequest,
	onDelta func(elelem.Delta) error,
) (elelem.Usage, error) {
	return f.Stream(ctx, req, onDelta)
}

func (f *fakeLLM) ListModels(_ context.Context) ([]string, error) {
	return []string{testModel}, nil
}

func (f *fakeLLM) Capabilities(_ elelem.Model) elelem.Capabilities {
	return elelem.Capabilities{
		SupportsToolChoice:          true,
		SupportsParallelToolCalls:   true,
		SupportsSamplingPenalties:   true,
		SupportsSamplingParams:      true,
		SupportsReasoningEffort:     true,
		SupportsDisablingReasoning:  true,
		SupportsStrictToolArguments: true,
		SupportsPromptCaching:       true,
		MaxReasoningEffort:          elelem.ReasoningEffortHigh,
	}
}

func (f *fakeLLM) TokenCounter() elelem.TokenCounter {
	return elelem.DefaultTokenCounter()
}

func (f *fakeLLM) Requests() []elelem.DriverRequest {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]elelem.DriverRequest(nil), f.requests...)
}

// syncBuf is a mutex-guarded log sink so the server's request goroutine can
// write while the test goroutine reads without a data race.
type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.b.Write(p)
}

func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.b.String()
}

type asyncStreamResult struct {
	status int
	body   string
	err    error
}

func testBox(t *testing.T) *secrets.Box {
	t.Helper()

	key := make([]byte, secrets.KeySize)
	for i := range key {
		key[i] = byte(i)
	}

	box, err := secrets.New(key)
	require.NoError(t, err)

	return box
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	return newTestServerWith(t, false)
}

// newTestServerWith builds the full HTTP stack against the real DB.
// passwordless toggles single-user auto-login. The fake LLM's client is
// wrapped with the usage recorder (persist=true) so a streamed round lands an
// llm_usage row, mirroring how the production service wires the registry.
func newTestServerWith(t *testing.T, passwordless bool) *httptest.Server {
	t.Helper()

	return newTestServerWithLLM(t, passwordless, &fakeLLM{})
}

func newTestServerWithLLM(
	t *testing.T,
	passwordless bool,
	llm *fakeLLM,
) *httptest.Server {
	t.Helper()

	return newTestServerWithLLMMode(t, passwordless, llm, false)
}

func newShowcaseTestServer(t *testing.T, llm *fakeLLM) *httptest.Server {
	t.Helper()

	return newTestServerWithLLMMode(t, false, llm, true)
}

func newTestServerWithLLMMode(
	t *testing.T,
	passwordless bool,
	llm *fakeLLM,
	showcaseMode bool,
) *httptest.Server {
	t.Helper()

	box := testBox(t)

	met, err := metrics.New()
	require.NoError(t, err)

	reg := upstreams.Discover(
		t.Context(),
		[]config.Upstream{{Name: "fake"}},
		func(config.Upstream) *elelem.Client {
			return elelem.New(usage.Wrap(llm, met, "chat", true))
		},
		time.Second,
		upstreams.NewHealthTracker([]string{"fake"}),
	)

	mgr := mcp.NewManager(box)
	readiness, err := operations.New(
		testInfra.Database,
		reg,
		operations.Config{
			AppVersion:       "test",
			DatabaseDriver:   db.DriverPostgres,
			BackupStatusPath: "/data/chatz-backup-status.json",
			BackupMaxAge:     time.Hour,
		},
	)
	require.NoError(t, err)

	srv := server.New(server.Deps{
		Auth:       auth.New(repositories.Q, passwordless),
		Chats:      chats.New(repositories.Q, reg, mgr, showcaseMode),
		MCP:        mgr,
		MCPServers: mcp.NewServerStore(repositories.Q),
		Models:     reg,
		Readiness:  readiness,
		Secrets:    box,
	})

	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	return ts
}

func resetDB(t *testing.T) {
	t.Helper()

	err := testInfra.PG.GormDB.Exec(
		"TRUNCATE users, sessions, chats, messages, mcp_servers, llm_usage " +
			"RESTART IDENTITY CASCADE",
	).Error
	require.NoError(t, err)
}

// newClient builds an http.Client with a fresh cookie jar so session cookies
// set by the server round-trip across requests, same as a browser.
func newClient(t *testing.T) *http.Client {
	t.Helper()

	jar, err := cookiejar.New(nil)
	require.NoError(t, err)

	return &http.Client{Jar: jar}
}

// newAuthedClient builds a cookie-jar client and bootstraps the first admin
// user on it, returning the client and the admin's id.
func newAuthedClient(
	t *testing.T,
	ts *httptest.Server,
) (*http.Client, string) {
	t.Helper()

	client := newClient(t)
	adminID := bootstrapAdmin(t, client, ts)

	return client, adminID
}

// installRejectAssistantTrigger installs a DB trigger that rejects any
// assistant-role message insert (simulating a completion-time persistence
// failure) and registers its removal via t.Cleanup.
func installRejectAssistantTrigger(t *testing.T) {
	t.Helper()

	require.NoError(
		t,
		testInfra.PG.GormDB.Exec(rejectAssistantMessageInsert).Error,
	)
	t.Cleanup(func() {
		require.NoError(
			t,
			testInfra.PG.GormDB.Exec(removeRejectAssistantMessageInsert).Error,
		)
	})
}

func buildReq(
	t *testing.T,
	method, url string,
	payload any,
) *http.Request {
	t.Helper()

	var body io.Reader

	if payload != nil {
		raw, err := json.Marshal(payload)
		require.NoError(t, err)

		body = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(t.Context(), method, url, body)
	require.NoError(t, err)

	if payload != nil {
		req.Header.Set(
			aichteeteapee.HeaderNameContentType,
			aichteeteapee.ContentTypeJSON,
		)
	}

	return req
}

// requestJSON does a request, decodes the JSON body into out (when non-nil),
// and returns the status. It closes the body.
func requestJSON(
	t *testing.T,
	client *http.Client,
	method, url string,
	payload, out any,
) int {
	t.Helper()

	resp, err := client.Do(buildReq(t, method, url, payload))
	require.NoError(t, err)

	defer func() { _ = resp.Body.Close() }()

	if out != nil {
		require.NoError(t, json.NewDecoder(resp.Body).Decode(out))
	}

	return resp.StatusCode
}

// requestStream POSTs and returns the status, content type, and the full body
// as a string. It closes the body. Every streaming endpoint here is a POST, so
// the method is not a parameter.
func requestStream(
	t *testing.T,
	client *http.Client,
	url string,
	payload any,
) (int, string, string) {
	t.Helper()

	resp, err := client.Do(buildReq(t, http.MethodPost, url, payload))
	require.NoError(t, err)

	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	ctype := resp.Header.Get(aichteeteapee.HeaderNameContentType)

	return resp.StatusCode, ctype, string(raw)
}

func executeStreamRequest(
	client *http.Client,
	req *http.Request,
) asyncStreamResult {
	resp, err := client.Do(req)
	if err != nil {
		return asyncStreamResult{
			err: ctxerrors.Wrap(err, "send stream request"),
		}
	}

	raw, readErr := io.ReadAll(resp.Body)

	closeErr := resp.Body.Close()
	if readErr != nil {
		return asyncStreamResult{
			status: resp.StatusCode,
			err:    ctxerrors.Wrap(readErr, "read stream response"),
		}
	}

	if closeErr != nil {
		return asyncStreamResult{
			status: resp.StatusCode,
			err:    ctxerrors.Wrap(closeErr, "close stream response"),
		}
	}

	return asyncStreamResult{
		status: resp.StatusCode,
		body:   string(raw),
	}
}

// findIdentityLog scans captured JSON log lines for a "request completed" line
// carrying user_id, returning it (or nil).
func findIdentityLog(buf *syncBuf) map[string]any {
	for _, l := range bytes.Split([]byte(buf.String()), []byte("\n")) {
		var rec map[string]any
		if json.Unmarshal(l, &rec) != nil {
			continue
		}

		if rec["msg"] == "request completed" {
			if _, ok := rec["user_id"]; ok {
				return rec
			}
		}
	}

	return nil
}

// authStatus GETs /auth/status with a fresh (cookieless) client and returns the
// decoded body plus whether a session cookie was set.
func authStatus(t *testing.T, ts *httptest.Server) (map[string]any, bool) {
	t.Helper()

	resp, err := (&http.Client{}).Do(
		buildReq(t, http.MethodGet, ts.URL+pathAuthStatus, nil),
	)
	require.NoError(t, err)

	defer func() { _ = resp.Body.Close() }()

	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

	setCookie := false

	for _, c := range resp.Cookies() {
		if c.Name == auth.CookieName && c.Value != "" {
			setCookie = true
		}
	}

	return body, setCookie
}

// bootstrapAdmin creates the first admin user (also logs it in on the given
// client via the session cookie) and returns its id.
func bootstrapAdmin(
	t *testing.T, client *http.Client, ts *httptest.Server,
) string {
	t.Helper()

	var user map[string]any

	status := requestJSON(
		t, client, http.MethodPost,
		ts.URL+pathAuthSetup,
		map[string]string{"username": adminUser, "password": adminPassword},
		&user,
	)
	require.Equal(t, http.StatusOK, status)

	id, ok := user["id"].(string)
	require.True(t, ok)

	return id
}

// seedChats inserts n chats for userID directly via the repo layer, each with
// a distinct UpdatedAt one second apart (oldest first) so List's
// UpdatedAt-desc order is deterministic across a multi-page walk. Each chat
// also gets one message — List() only returns chats with at least one
// message (see chats.Service.List / GetOrCreateEmpty), so a message-less seed
// here would be invisible to every assertion below. Returns the ids in the
// order List should return them (newest first).
func seedChats(t *testing.T, userID uuid.UUID, n int) []string {
	t.Helper()

	base := time.Now().Add(-time.Hour)
	ids := make([]string, n)

	for i := range n {
		chat := &models.Chat{
			UserID:  userID,
			Title:   fmt.Sprintf("chat-%02d", i),
			ModelID: testModel,
		}
		require.NoError(
			t, repositories.Chat.WithContext(t.Context()).Create(chat),
		)

		turnComplete := true
		msg := &models.Message{
			ChatID:       chat.ID,
			TurnID:       uuid.New(),
			TurnComplete: &turnComplete,
			Role:         models.MessageRoleUser,
			Content:      "seed",
		}
		require.NoError(
			t, repositories.Message.WithContext(t.Context()).Create(msg),
		)

		updatedAt := base.Add(time.Duration(i) * time.Second)
		_, err := repositories.Chat.WithContext(t.Context()).
			Where(repositories.Chat.ID.Eq(chat.ID)).
			UpdateColumn(repositories.Chat.UpdatedAt, updatedAt)
		require.NoError(t, err)

		ids[n-1-i] = chat.ID.String()
	}

	return ids
}

// seedMessages inserts n visible (non-empty content) messages for chatID
// directly via the repo layer, each with a distinct CreatedAt one second
// apart so ListMessages' CreatedAt-asc order is deterministic. Returns the
// ids in oldest-first order (the order ListMessages should return them).
func seedMessages(t *testing.T, chatID uuid.UUID, n int) []string {
	t.Helper()

	base := time.Now().Add(-time.Hour)
	ids := make([]string, n)

	for i := range n {
		turnComplete := true
		msg := &models.Message{
			ChatID:       chatID,
			TurnID:       uuid.New(),
			TurnComplete: &turnComplete,
			Role:         models.MessageRoleUser,
			Content:      fmt.Sprintf("msg-%02d", i),
		}
		require.NoError(
			t, repositories.Message.WithContext(t.Context()).Create(msg),
		)

		createdAt := base.Add(time.Duration(i) * time.Second)
		_, err := repositories.Message.WithContext(t.Context()).
			Where(repositories.Message.ID.Eq(msg.ID)).
			UpdateColumn(repositories.Message.CreatedAt, createdAt)
		require.NoError(t, err)

		ids[i] = msg.ID.String()
	}

	return ids
}

// pagedURL builds a list URL with limit/offset query params.
func pagedURL(base string, limit, offset int) string {
	q := url.Values{}
	q.Set("limit", fmt.Sprint(limit))
	q.Set("offset", fmt.Sprint(offset))

	return base + "?" + q.Encode()
}

// idsOf extracts the "id" field from a slice of item maps, in order.
func idsOf(t *testing.T, items []any) []string {
	t.Helper()

	out := make([]string, len(items))

	for i, raw := range items {
		m, ok := raw.(map[string]any)
		require.True(t, ok)

		id, ok := m["id"].(string)
		require.True(t, ok)

		out[i] = id
	}

	return out
}

// assertPaginates walks the canonical 5-case page table (page1 / page2 /
// partial last page / past end / way past end) against baseURL, asserting
// per-page item ids AND that total stays len(wantIDs) at every offset. limit
// is the page size; wantIDs is the full expected id order.
func assertPaginates(
	t *testing.T,
	client *http.Client,
	baseURL string,
	wantIDs []string,
	limit int,
) {
	t.Helper()

	total := len(wantIDs)

	testCases := []struct {
		name      string
		offset    int
		wantCount int
	}{
		{"page 1", 0, limit},
		{"page 2", limit, limit},
		{"page 3 (partial)", 2 * limit, total - 2*limit},
		{"past end", total, 0},
		{"way past end", total + 87, 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var env map[string]any

			status := requestJSON(t, client, http.MethodGet,
				pagedURL(baseURL, limit, tc.offset), nil, &env)
			require.Equal(t, http.StatusOK, status)

			items, ok := env["items"].([]any)
			require.True(t, ok)
			require.Len(t, items, tc.wantCount)
			assert.InDelta(t, float64(total), env["total"], 0,
				"total must stay %d regardless of offset", total)
			assert.InDelta(t, float64(limit), env["limit"], 0)
			assert.InDelta(t, float64(tc.offset), env["offset"], 0)

			gotIDs := idsOf(t, items)

			end := tc.offset + tc.wantCount
			if end > len(wantIDs) {
				end = len(wantIDs)
			}

			wantSlice := []string{}
			if tc.offset < len(wantIDs) {
				wantSlice = wantIDs[tc.offset:end]
			}

			assert.Equal(t, wantSlice, gotIDs)
		})
	}
}

// seedTurns inserts the given physical message rows directly via the repo
// layer (mirroring exactly what persistTurn writes), used to set up
// multi-turn conversation history for the Continue tests.
func seedTurns(t *testing.T, rows []*models.Message) {
	t.Helper()

	for _, row := range rows {
		require.NoError(
			t,
			repositories.Message.WithContext(t.Context()).Create(row),
		)
	}
}

// seedToolTurn builds and seeds a complete single-turn tool-calling exchange
// (user -> assistant tool call -> tool result -> assistant answer) for chatID,
// returning the shared turn id. Used by
// TestServer_Continue_ReplaysPersistedToolHistory to set up prior history
// before asking the model "what happened earlier?".
func seedToolTurn(
	t *testing.T,
	chatID uuid.UUID,
	result string,
	resultIsErr bool,
) uuid.UUID {
	t.Helper()

	callsRaw, err := json.Marshal([]storedToolCallSeed{
		{
			ID:        "call_state",
			Name:      "get_state",
			Arguments: `{"scope":"current"}`,
		},
	})
	require.NoError(t, err)

	turnID := uuid.New()
	turnComplete := true

	priorMessages := []*models.Message{
		{
			ChatID:       chatID,
			TurnID:       turnID,
			TurnComplete: &turnComplete,
			Role:         models.MessageRoleUser,
			Content:      "check the current state",
		},
		{
			ChatID:       chatID,
			TurnID:       turnID,
			TurnComplete: &turnComplete,
			Role:         models.MessageRoleAssistant,
			ToolCalls:    datatypes.JSON(callsRaw),
		},
		{
			ChatID:       chatID,
			TurnID:       turnID,
			TurnComplete: &turnComplete,
			Role:         models.MessageRoleTool,
			Content:      result,
			ToolCallID:   "call_state",
			IsError:      resultIsErr,
		},
		{
			ChatID:       chatID,
			TurnID:       turnID,
			TurnComplete: &turnComplete,
			Role:         models.MessageRoleAssistant,
			Content:      "The state check completed.",
		},
	}

	seedTurns(t, priorMessages)

	messageRepo := repositories.Message
	_, err = messageRepo.WithContext(t.Context()).
		Where(messageRepo.ChatID.Eq(chatID)).
		Update(messageRepo.CreatedAt, time.Unix(1, 0).UTC())
	require.NoError(t, err)

	return turnID
}

type storedToolCallSeed struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}
