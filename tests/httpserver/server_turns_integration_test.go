//go:build integration

package httpservertest

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/psyb0t/chatz/internal/pkg/db/models"
	"github.com/psyb0t/chatz/internal/pkg/db/repositories"
	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/elelem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

// seedMessage builds the shape a replayed history row takes once it reaches
// elelem: text content, stamped as seed rather than as this turn's own.
func seedMessage(role elelem.Role, text string) elelem.Message {
	return elelem.Message{
		Role:    role,
		Content: elelem.Text(text),
		Origin:  elelem.MessageOriginSeed,
	}
}

func TestServer_Continue_ReplaysPersistedToolHistory(t *testing.T) {
	testCases := []struct {
		name        string
		result      string
		resultIsErr bool
	}{
		{"successful tool result", `{"status":"ready"}`, false},
		{"failed tool result", "tool failed: permission denied", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resetDB(t)

			llm := &fakeLLM{}
			ts := newTestServerWithLLM(t, false, llm)

			client, adminID := newAuthedClient(t, ts)

			chat := &models.Chat{
				UserID:  uuid.MustParse(adminID),
				Title:   "tool history",
				ModelID: testModel,
			}
			require.NoError(
				t,
				repositories.Chat.WithContext(t.Context()).Create(chat),
			)

			seedToolTurn(t, chat.ID, tc.result, tc.resultIsErr)

			status, _, body := requestStream(
				t,
				client,
				ts.URL+pathChatsPrefix+chat.ID.String(),
				map[string]string{
					"model":   testModel,
					"message": "what happened earlier?",
				},
			)
			require.Equal(t, http.StatusOK, status)
			assert.Contains(t, body, fakeReply)

			requests := llm.Requests()
			require.Len(t, requests, 1)
			require.Len(t, requests[0].Messages, 6)
			assert.Equal(t, elelem.RoleSystem, requests[0].Messages[0].Role)
			assert.Equal(t, []elelem.Message{
				{
					Role:    elelem.RoleUser,
					Content: elelem.Text("check the current state"),
					Origin:  elelem.MessageOriginSeed,
				},
				{
					Role: elelem.RoleAssistant,
					ToolCalls: []elelem.ToolCall{
						{
							ID:        "call_state",
							Name:      "get_state",
							Arguments: json.RawMessage(`{"scope":"current"}`),
						},
					},
					Origin: elelem.MessageOriginSeed,
				},
				{
					Role:              elelem.RoleTool,
					Content:           elelem.Text(tc.result),
					ToolCallID:        "call_state",
					ToolResultIsError: tc.resultIsErr,
					Origin:            elelem.MessageOriginSeed,
				},
				{
					Role:    elelem.RoleAssistant,
					Content: elelem.Text("The state check completed."),
					Origin:  elelem.MessageOriginSeed,
				},
				{
					Role:    elelem.RoleUser,
					Content: elelem.Text("what happened earlier?"),
					Origin:  elelem.MessageOriginSeed,
				},
			}, requests[0].Messages[1:])
		})
	}
}

func TestServer_Continue_GroupsAndQuarantinesDurableTurns(t *testing.T) {
	resetDB(t)

	llm := &fakeLLM{}
	ts := newTestServerWithLLM(t, false, llm)

	client, adminID := newAuthedClient(t, ts)
	chat := &models.Chat{
		UserID:  uuid.MustParse(adminID),
		Title:   "durable turns",
		ModelID: testModel,
	}
	require.NoError(
		t,
		repositories.Chat.WithContext(t.Context()).Create(chat),
	)

	turnA := uuid.New()
	turnB := uuid.New()
	invalidTurn := uuid.New()
	incompleteTurn := uuid.New()
	laterTurn := uuid.New()
	complete := true
	incomplete := false
	missingResultCalls, err := json.Marshal([]storedToolCallSeed{
		{
			ID:        "call-missing-result",
			Name:      "lookup",
			Arguments: `{}`,
		},
	})
	require.NoError(t, err)

	physicalRows := []*models.Message{
		{
			ChatID:       chat.ID,
			TurnID:       turnA,
			TurnComplete: &complete,
			Role:         models.MessageRoleUser,
			Content:      "turn a user",
		},
		{
			ChatID:       chat.ID,
			TurnID:       turnB,
			TurnComplete: &complete,
			Role:         models.MessageRoleUser,
			Content:      "turn b user",
		},
		{
			ChatID:       chat.ID,
			TurnID:       turnA,
			TurnComplete: &complete,
			Role:         models.MessageRoleAssistant,
			Content:      "turn a answer",
		},
		{
			ChatID:       chat.ID,
			TurnID:       turnB,
			TurnComplete: &complete,
			Role:         models.MessageRoleAssistant,
			Content:      "turn b answer",
		},
		{
			ChatID:       chat.ID,
			TurnID:       invalidTurn,
			TurnComplete: &complete,
			Role:         models.MessageRoleUser,
			Content:      "invalid turn user",
		},
		{
			ChatID:       chat.ID,
			TurnID:       invalidTurn,
			TurnComplete: &complete,
			Role:         models.MessageRoleAssistant,
			ToolCalls:    datatypes.JSON(missingResultCalls),
		},
		{
			ChatID:       chat.ID,
			TurnID:       incompleteTurn,
			TurnComplete: &incomplete,
			Role:         models.MessageRoleUser,
			Content:      "hanging attempt",
		},
		{
			ChatID:       chat.ID,
			TurnID:       incompleteTurn,
			TurnComplete: &incomplete,
			Role:         models.MessageRoleAssistant,
			Content:      "partial hanging answer",
		},
		{
			ChatID:       chat.ID,
			TurnID:       laterTurn,
			TurnComplete: &complete,
			Role:         models.MessageRoleUser,
			Content:      "later valid user",
		},
		{
			ChatID:       chat.ID,
			TurnID:       laterTurn,
			TurnComplete: &complete,
			Role:         models.MessageRoleAssistant,
			Content:      "later valid answer",
		},
	}

	seedTurns(t, physicalRows)

	status, _, body := requestStream(
		t,
		client,
		ts.URL+pathChatsPrefix+chat.ID.String(),
		map[string]string{
			"model":   testModel,
			"message": "continue now",
		},
	)
	require.Equal(t, http.StatusOK, status)
	assert.Contains(t, body, fakeReply)

	requests := llm.Requests()
	require.Len(t, requests, 1)
	require.Len(t, requests[0].Messages, 8)
	assert.Equal(t, elelem.RoleSystem, requests[0].Messages[0].Role)
	assert.Equal(t, []elelem.Message{
		seedMessage(elelem.RoleUser, "turn a user"),
		seedMessage(elelem.RoleAssistant, "turn a answer"),
		seedMessage(elelem.RoleUser, "turn b user"),
		seedMessage(elelem.RoleAssistant, "turn b answer"),
		seedMessage(elelem.RoleUser, "later valid user"),
		seedMessage(elelem.RoleAssistant, "later valid answer"),
		seedMessage(elelem.RoleUser, "continue now"),
	}, requests[0].Messages[1:])

	var messagesEnv map[string]any

	status = requestJSON(
		t,
		client,
		http.MethodGet,
		ts.URL+pathChatsPrefix+chat.ID.String()+"/messages",
		nil,
		&messagesEnv,
	)
	require.Equal(t, http.StatusOK, status)
	assert.InDelta(t, float64(12), messagesEnv["total"], 0)

	items, ok := messagesEnv["items"].([]any)
	require.True(t, ok)

	foundPendingUser := false
	foundCheckpoint := false

	for _, raw := range items {
		message, ok := raw.(map[string]any)
		require.True(t, ok)

		if message["content"] == "hanging attempt" {
			foundPendingUser = true
		}

		if message["content"] == "partial hanging answer" {
			foundCheckpoint = true

			assert.Equal(t, true, message["incomplete"])
		}
	}

	assert.True(t, foundPendingUser)
	assert.True(t, foundCheckpoint)
}

func TestServer_Create_ReplacesAssistantCheckpointOnComplete(t *testing.T) {
	resetDB(t)

	deltaSent := make(chan struct{})
	release := make(chan struct{})

	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})

	llm := &fakeLLM{
		stream: func(
			ctx context.Context,
			_ int,
			_ elelem.DriverRequest,
			onDelta func(elelem.Delta) error,
		) (elelem.Usage, error) {
			delta := elelem.Delta{Text: "partial answer"}
			if err := onDelta(delta); err != nil {
				return elelem.Usage{}, err
			}

			close(deltaSent)

			select {
			case <-ctx.Done():
				return elelem.Usage{}, ctxerrors.Wrap(
					ctx.Err(),
					"wait to finish",
				)
			case <-release:
			}

			return elelem.Usage{
				TokenCounts:  elelem.TokenCounts{Completion: 2, Total: 2},
				FinishReason: elelem.FinishReasonStop,
			}, nil
		},
	}
	ts := newTestServerWithLLM(t, false, llm)
	client, adminID := newAuthedClient(t, ts)
	turnCtx, cancelTurn := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancelTurn)

	request := buildReq(
		t,
		http.MethodPost,
		ts.URL+pathChats,
		map[string]string{
			"model":   testModel,
			"message": "save this while it streams",
		},
	).WithContext(turnCtx)
	result := make(chan asyncStreamResult, 1)

	go func() {
		result <- executeStreamRequest(client, request)
	}()

	select {
	case <-deltaSent:
	case <-turnCtx.Done():
		require.NoError(t, turnCtx.Err())
	}

	chatRepo := repositories.Chat
	chat, err := chatRepo.WithContext(t.Context()).
		Where(chatRepo.UserID.Eq(uuid.MustParse(adminID))).
		First()
	require.NoError(t, err)

	messageRepo := repositories.Message
	checkpointRows, err := messageRepo.WithContext(t.Context()).
		Where(messageRepo.ChatID.Eq(chat.ID)).
		Order(messageRepo.Position).
		Find()
	require.NoError(t, err)
	require.Len(t, checkpointRows, 2)
	assert.Equal(t, models.MessageRoleUser, checkpointRows[0].Role)
	assert.Equal(t, models.MessageRoleAssistant, checkpointRows[1].Role)
	assert.Equal(t, "partial answer", checkpointRows[1].Content)
	require.NotNil(t, checkpointRows[1].TurnComplete)
	assert.False(t, *checkpointRows[1].TurnComplete)

	close(release)

	select {
	case completed := <-result:
		require.NoError(t, completed.err)
		require.Equal(t, http.StatusOK, completed.status)
		assert.Contains(t, completed.body, "partial answer")
	case <-turnCtx.Done():
		require.NoError(t, turnCtx.Err())
	}

	completedRows, err := messageRepo.WithContext(t.Context()).
		Where(messageRepo.ChatID.Eq(chat.ID)).
		Order(messageRepo.Position).
		Find()
	require.NoError(t, err)
	require.Len(t, completedRows, 2)
	assert.Equal(t, models.MessageRoleAssistant, completedRows[1].Role)
	assert.Equal(t, "partial answer", completedRows[1].Content)
	require.NotNil(t, completedRows[1].TurnComplete)
	assert.True(t, *completedRows[1].TurnComplete)

	var messagesEnv map[string]any

	status := requestJSON(
		t,
		client,
		http.MethodGet,
		ts.URL+pathChatsPrefix+chat.ID.String()+"/messages",
		nil,
		&messagesEnv,
	)
	require.Equal(t, http.StatusOK, status)

	items, ok := messagesEnv["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 2)
	assistant, ok := items[1].(map[string]any)
	require.True(t, ok)
	assert.Nil(t, assistant["incomplete"])
}

func TestServer_CreateSerializesFollowingContinuation(t *testing.T) {
	resetDB(t)

	firstEntered := make(chan elelem.DriverRequest, 1)
	secondEntered := make(chan elelem.DriverRequest, 1)
	releaseFirst := make(chan struct{})

	var releaseFirstOnce sync.Once

	release := func() {
		releaseFirstOnce.Do(func() {
			close(releaseFirst)
		})
	}
	t.Cleanup(release)

	llm := &fakeLLM{
		stream: func(
			ctx context.Context,
			callIndex int,
			req elelem.DriverRequest,
			onDelta func(elelem.Delta) error,
		) (elelem.Usage, error) {
			switch callIndex {
			case 0:
				firstEntered <- req

				select {
				case <-ctx.Done():
					return elelem.Usage{}, ctxerrors.Wrap(
						ctx.Err(),
						"wait to release first request",
					)
				case <-releaseFirst:
				}
			case 1:
				secondEntered <- req
			}

			if err := onDelta(elelem.Delta{Text: fakeReply}); err != nil {
				return elelem.Usage{}, err
			}

			return elelem.Usage{
				TokenCounts:  elelem.TokenCounts{Completion: 3, Total: 3},
				FinishReason: elelem.FinishReasonStop,
			}, nil
		},
	}
	ts := newTestServerWithLLM(t, false, llm)

	client := newClient(t)
	adminID := uuid.MustParse(bootstrapAdmin(t, client, ts))
	overallCtx, cancelOverall := context.WithTimeout(
		t.Context(),
		5*time.Second,
	)
	t.Cleanup(cancelOverall)

	createReq := buildReq(
		t,
		http.MethodPost,
		ts.URL+pathChats,
		map[string]string{
			"model":   testModel,
			"message": "first message",
		},
	)
	createResult := make(chan asyncStreamResult, 1)

	go func() {
		createResult <- executeStreamRequest(client, createReq)
	}()

	select {
	case <-firstEntered:
	case <-overallCtx.Done():
		require.NoError(t, overallCtx.Err())
	}

	chatRepo := repositories.Chat
	chat, err := chatRepo.WithContext(t.Context()).
		Where(chatRepo.UserID.Eq(adminID)).
		First()
	require.NoError(t, err)

	continueReq := buildReq(
		t,
		http.MethodPost,
		ts.URL+pathChatsPrefix+chat.ID.String(),
		map[string]string{
			"model":   testModel,
			"message": "second message",
		},
	)
	continueResult := make(chan asyncStreamResult, 1)

	go func() {
		continueResult <- executeStreamRequest(client, continueReq)
	}()

	observationCtx, cancelObservation := context.WithTimeout(
		t.Context(),
		100*time.Millisecond,
	)
	select {
	case req := <-secondEntered:
		cancelObservation()
		require.Failf(
			t,
			"second request entered LLM early",
			"messages: %#v",
			req.Messages,
		)
	case <-observationCtx.Done():
		require.ErrorIs(t, observationCtx.Err(), context.DeadlineExceeded)
	}

	cancelObservation()

	release()

	select {
	case result := <-createResult:
		require.NoError(t, result.err)
		require.Equal(t, http.StatusOK, result.status)
		assert.Contains(t, result.body, fakeReply)
	case <-overallCtx.Done():
		require.NoError(t, overallCtx.Err())
	}

	var secondRequest elelem.DriverRequest
	select {
	case secondRequest = <-secondEntered:
	case <-overallCtx.Done():
		require.NoError(t, overallCtx.Err())
	}

	require.Len(t, secondRequest.Messages, 4)
	assert.Equal(t, elelem.RoleSystem, secondRequest.Messages[0].Role)
	assert.Equal(t, []elelem.Message{
		seedMessage(elelem.RoleUser, "first message"),
		seedMessage(elelem.RoleAssistant, fakeReply),
		seedMessage(elelem.RoleUser, "second message"),
	}, secondRequest.Messages[1:])

	select {
	case result := <-continueResult:
		require.NoError(t, result.err)
		require.Equal(t, http.StatusOK, result.status)
		assert.Contains(t, result.body, fakeReply)
	case <-overallCtx.Done():
		require.NoError(t, overallCtx.Err())
	}
}

func TestServer_Continue_CompletionFailureRetainsVisibleUserMessage(
	t *testing.T,
) {
	resetDB(t)

	installRejectAssistantTrigger(t)

	llm := &fakeLLM{
		stream: func(
			_ context.Context,
			callIndex int,
			_ elelem.DriverRequest,
			onDelta func(elelem.Delta) error,
		) (elelem.Usage, error) {
			if callIndex == 0 {
				err := onDelta(elelem.Delta{
					ToolCall: &elelem.ToolCallDelta{
						Index:     0,
						ID:        "call-missing",
						Name:      "missing__tool",
						Arguments: `{}`,
					},
				})

				return elelem.Usage{
					FinishReason: elelem.FinishReasonToolCalls,
				}, err
			}

			err := onDelta(elelem.Delta{Text: fakeReply})

			return elelem.Usage{FinishReason: elelem.FinishReasonStop}, err
		},
	}
	ts := newTestServerWithLLM(t, false, llm)

	client, adminID := newAuthedClient(t, ts)
	chat := &models.Chat{
		UserID:  uuid.MustParse(adminID),
		Title:   "atomic tool turn",
		ModelID: testModel,
	}
	require.NoError(
		t,
		repositories.Chat.WithContext(t.Context()).Create(chat),
	)

	chatRepo := repositories.Chat
	before, err := chatRepo.WithContext(t.Context()).
		Where(chatRepo.ID.Eq(chat.ID)).
		First()
	require.NoError(t, err)

	status, _, body := requestStream(
		t,
		client,
		ts.URL+pathChatsPrefix+chat.ID.String(),
		map[string]string{
			"model":   testModel,
			"message": "run the missing tool",
		},
	)
	require.Equal(t, http.StatusOK, status)
	assert.Contains(t, body, fakeReply)
	require.Len(t, llm.Requests(), 2)

	messageRepo := repositories.Message
	rows, err := messageRepo.WithContext(t.Context()).
		Where(messageRepo.ChatID.Eq(chat.ID)).
		Find()
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, models.MessageRoleUser, rows[0].Role)
	assert.Equal(t, "run the missing tool", rows[0].Content)
	require.NotNil(t, rows[0].TurnComplete)
	assert.False(t, *rows[0].TurnComplete)
	assert.NotEqual(t, uuid.Nil, rows[0].TurnID)

	after, err := chatRepo.WithContext(t.Context()).
		Where(chatRepo.ID.Eq(chat.ID)).
		First()
	require.NoError(t, err)
	assert.True(t, after.UpdatedAt.After(before.UpdatedAt))

	var messagesEnv map[string]any

	status = requestJSON(
		t,
		client,
		http.MethodGet,
		ts.URL+pathChatsPrefix+chat.ID.String()+"/messages",
		nil,
		&messagesEnv,
	)
	require.Equal(t, http.StatusOK, status)
	assert.InDelta(t, float64(1), messagesEnv["total"], 0)

	items, ok := messagesEnv["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 1)

	message, ok := items[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "user", message["role"])
	assert.Equal(t, "run the missing tool", message["content"])

	var chatsEnv map[string]any

	status = requestJSON(
		t, client, http.MethodGet, ts.URL+pathChats, nil, &chatsEnv,
	)
	require.Equal(t, http.StatusOK, status)
	assert.InDelta(t, float64(1), chatsEnv["total"], 0)
}
