package chats

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	chatcore "github.com/psyb0t/chatz/internal/pkg/core/chat"
	"github.com/psyb0t/chatz/internal/pkg/core/chats/fixedresponses"
	"github.com/psyb0t/chatz/internal/pkg/core/chats/prompts"
	"github.com/psyb0t/chatz/internal/pkg/db/models"
	"github.com/psyb0t/chatz/internal/pkg/db/repositories"
	"github.com/psyb0t/chatz/internal/pkg/tiktoken"
	"github.com/psyb0t/chatz/internal/pkg/usage"
	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/ctxscope"
	"github.com/psyb0t/elelem"
	"github.com/psyb0t/essessey"
	essesseysse "github.com/psyb0t/essessey/sse"
	"gorm.io/datatypes"
)

const (
	// baseSystemPrompt is the base instruction for a chat turn. The generated
	// GenUI (json-render) component-catalog guide is ALWAYS appended to it by
	// chatSystemPrompt, so the model emits ```spec UI blocks instead of
	// markdown/HTML.
	baseSystemPrompt = "You are chatz, a helpful AI assistant. " +
		"Answer concisely and use the available tools when they help."

	persistTimeout = 10 * time.Second
	maxTitleRunes  = 60

	// demoTokenDelay paces one token per SSE delta so a canned demo streams
	// progressively — like a live model turn — instead of landing in one burst.
	demoTokenDelay = 16 * time.Millisecond

	// demoChunkRunes is the streamed slice size for a scripted showcase turn —
	// roughly one word per delta so the client sees text arrive token-by-token,
	// like a real model turn rather than in multi-word chunks.
	demoChunkRunes = 6

	// demoChunkDelay paces those deltas so the scripted demo streams
	// progressively instead of landing in one burst.
	demoChunkDelay = 22 * time.Millisecond

	// demoToolLatency is a short pause between a scripted tool_use and its
	// tool_result, so the client visibly renders the CALLING state before the
	// result lands — a real tool call is never instantaneous.
	demoToolLatency = 300 * time.Millisecond

	// demoToolBlocks is how many content blocks one scripted tool step
	// consumes: a tool_use block plus its tool_result block.
	demoToolBlocks = 2
)

// chatSystemPrompt returns the effective system prompt for a chat turn: the
// base instruction with the generated GenUI guide appended.
func chatSystemPrompt() string {
	return baseSystemPrompt + "\n\n" + prompts.GenUIInstructions()
}

// streamTurn runs one chat turn in a goroutine, streaming SSE frames into a
// pipe whose reader becomes the response body. The turn's new messages are
// persisted (on a detached ctx) before the stream closes.
func (s *Service) streamTurn(
	ctx context.Context,
	chat *models.Chat,
	client *elelem.Client,
	seed elelem.Prompt,
	turnID uuid.UUID,
	onDone func(),
) io.Reader {
	// Stamp the owning chat + user on the ctx so the usage recorder can
	// attribute every streamed round's llm_usage row to them.
	ctx = usage.WithAttribution(ctx, chat.ID, chat.UserID)

	pr, pw := io.Pipe()

	go s.runStreamTurn(ctx, chat, client, seed, turnID, onDone, pw)

	return pr
}

func (s *Service) runStreamTurn(
	ctx context.Context,
	chat *models.Chat,
	client *elelem.Client,
	seed elelem.Prompt,
	turnID uuid.UUID,
	onDone func(),
	pw *io.PipeWriter,
) {
	if onDone != nil {
		defer onDone()
	}

	defer func() { _ = pw.Close() }()
	defer func() {
		if r := recover(); r != nil {
			ctxscope.GetLogger(ctx).Error(
				"chat stream panic",
				"chat_id", chat.ID,
				"reason", "chat_stream_panic",
			)
		}
	}()

	pub := essessey.NewPublisher(ctx, essesseysse.NewWriterSink(pw))

	checkpoint := newTurnCheckpoint(s, chat.ID, turnID, chat.ModelID)

	result, err := s.runStreamCandidates(
		ctx,
		chat,
		client,
		seed,
		pub,
		checkpoint,
	)
	if err == nil {
		checkpoint.flush(ctx)
		s.persistTurn(ctx, chat.ID, turnID, result)

		return
	}

	handleStreamTurnFailure(ctx, chat.ID, pub, err)
}

func (s *Service) runStreamCandidates(
	ctx context.Context,
	chat *models.Chat,
	client *elelem.Client,
	seed elelem.Prompt,
	pub *essessey.Publisher,
	checkpoint *turnCheckpoint,
) (*chatcore.Result, error) {
	candidates := s.streamCandidates(chat.ModelID, client)

	var finalErr error

	for candidateIndex, candidate := range candidates {
		if err := publishTurnStatus(pub, turnStatusConnecting); err != nil {
			return nil, err
		}

		if checkpoint != nil {
			checkpoint.modelID = candidate.modelID
		}

		var onAssistantDelta func(context.Context, elelem.Delta)
		if checkpoint != nil {
			onAssistantDelta = checkpoint.add
		}

		result, err := s.runStreamCandidate(
			ctx,
			chat,
			seed,
			pub,
			candidate,
			onAssistantDelta,
		)
		if err == nil {
			return result, nil
		}

		finalErr = err
		if chatcore.StreamStarted(err) ||
			!isFallbackEligible(err) ||
			candidateIndex == len(candidates)-1 {
			break
		}

		ctxscope.GetLogger(ctx).Warn(
			"retrying chat turn with fallback model",
			"chat_id", chat.ID,
			"failed_model", candidate.modelID,
			"fallback_model", candidates[candidateIndex+1].modelID,
			"err", err,
			"reason", "pre_stream_transient_provider_failure",
		)

		if err := publishTurnStatus(pub, turnStatusRetrying); err != nil {
			return nil, err
		}
	}

	return nil, finalErr
}

type streamCandidate struct {
	modelID string
	client  *elelem.Client
}

func (s *Service) streamCandidates(
	modelID string,
	client *elelem.Client,
) []streamCandidate {
	if s.models == nil {
		return []streamCandidate{{modelID: modelID, client: client}}
	}

	registryCandidates := s.models.CandidatesFor(modelID)
	if len(registryCandidates) == 0 {
		return []streamCandidate{{modelID: modelID, client: client}}
	}

	candidates := make([]streamCandidate, 0, len(registryCandidates))
	for _, candidate := range registryCandidates {
		candidates = append(candidates, streamCandidate{
			modelID: candidate.ModelID,
			client:  candidate.Client,
		})
	}

	return candidates
}

func (s *Service) runStreamCandidate(
	ctx context.Context,
	chat *models.Chat,
	seed elelem.Prompt,
	pub *essessey.Publisher,
	candidate streamCandidate,
	onAssistantDelta func(context.Context, elelem.Delta),
) (*chatcore.Result, error) {
	runChat := *chat
	runChat.ModelID = candidate.modelID

	result, err := chatcore.Run(ctx, chatcore.Deps{
		Client:    candidate.client,
		Tools:     s.toolExecutorFor(ctx, chat),
		Publisher: pub,
	}, chatcore.Request{
		MessageID:        newMessageID(),
		ConversationID:   chat.ID.String(),
		Model:            elelem.Model{ID: candidate.modelID},
		Prompt:           seed,
		MaxContextTokens: historyTokenBudget(chat),
		Params:           genParams(chat),
		OnAssistantDelta: onAssistantDelta,
		OnFirstDelta: func(_ context.Context) error {
			return publishTurnStatus(pub, turnStatusStreaming)
		},
		OnToolCallStart: func(_ context.Context, _ elelem.ToolCallEvent) error {
			return publishTurnStatus(pub, turnStatusRunningTool)
		},
		OnRoundStart: func(
			callbackCtx context.Context,
			event *elelem.RoundEvent,
		) error {
			if err := publishTurnStatus(
				pub,
				turnStatusWaitingForFirstToken,
			); err != nil {
				return err
			}

			logOutboundPrompt(
				callbackCtx,
				&runChat,
				event.Round,
				event.Messages,
			)

			return nil
		},
	})
	if err != nil {
		return nil, ctxerrors.Wrap(err, "run candidate model")
	}

	return result, nil
}

func handleStreamTurnFailure(
	ctx context.Context,
	chatID uuid.UUID,
	pub *essessey.Publisher,
	err error,
) {
	failure, publishable := streamFailure(err)
	if !publishable {
		ctxscope.GetLogger(ctx).Info(
			"chat turn stopped",
			"chat_id", chatID,
			"reason", "chat_turn_cancelled",
		)

		return
	}

	if publishErr := publishStreamFailure(pub, failure); publishErr != nil {
		ctxscope.GetLogger(ctx).Warn(
			"chat turn terminal event was not delivered",
			"chat_id", chatID,
			"failure_type", failure.Error.Type,
			"reason", "chat_turn_terminal_event_failed",
		)
	}

	ctxscope.GetLogger(ctx).Error(
		"chat turn failed",
		"chat_id", chatID,
		"failure_type", failure.Error.Type,
		"reason", "chat_turn_failed",
	)
}

// firstTurnBody streams either the canned showcase response or the first LLM
// turn for a freshly-created chat. Both persist the user message first; a
// showcase turn additionally persists its assistant output like a real turn.
func (s *Service) firstTurnBody(
	ctx context.Context,
	chat *models.Chat,
	userMessage string,
	isDemo bool,
	demo fixedresponses.Response,
	client *elelem.Client,
) (io.Reader, error) {
	if isDemo {
		return s.demoTurn(ctx, chat, userMessage, demo)
	}

	unlockTurn, err := lockChatTurn(ctx, chat.ID)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "lock chat turn")
	}

	defer releaseUntransferredChatTurn(&unlockTurn)

	seed := buildPrompt(
		chatSystemPrompt(),
		nil,
		userMessage,
	)

	turnID := uuid.New()
	if err := s.savePendingUserMessage(
		ctx,
		chat.ID,
		turnID,
		userMessage,
	); err != nil {
		return nil, ctxerrors.Wrap(err, "save user message")
	}

	body := s.streamTurn(ctx, chat, client, seed, turnID, unlockTurn)
	unlockTurn = nil

	return body, nil
}

// nextTurnBody streams either the canned showcase response or the next LLM turn
// for an existing chat. Both persist the user message first; a showcase turn
// additionally persists its assistant output like a real turn.
func (s *Service) nextTurnBody(
	ctx context.Context,
	chat *models.Chat,
	userMessage string,
	isDemo bool,
	demo fixedresponses.Response,
	client *elelem.Client,
	onDone func(),
) (io.Reader, error) {
	if isDemo {
		return s.demoTurn(ctx, chat, userMessage, demo)
	}

	// History is loaded BEFORE persisting the new user message so the seed's
	// trailing user turn isn't duplicated by the just-saved row.
	history, err := s.loadHistory(ctx, chat)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "load history")
	}

	systemMessage := chatSystemPrompt()

	promptContext, err := selectPromptHistory(
		systemMessage,
		history,
		userMessage,
		historyTokenBudget(chat),
	)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "select prompt history")
	}

	seed := buildPrompt(systemMessage, promptContext.History, userMessage)

	turnID := uuid.New()
	if err := s.savePendingUserMessage(
		ctx,
		chat.ID,
		turnID,
		userMessage,
	); err != nil {
		return nil, ctxerrors.Wrap(err, "save user message")
	}

	return s.streamTurn(ctx, chat, client, seed, turnID, onDone), nil
}

// demoTurn streams a canned showcase response. Every showcase turn is durable
// (persisted like a real turn) so a recording can be refreshed and continued: a
// KindTools reply replays scripted thinking + tool cards + a dashboard, while
// the KindText fallback (a showcase file that failed to load) streams its text.
func (s *Service) demoTurn(
	ctx context.Context,
	chat *models.Chat,
	userMessage string,
	demo fixedresponses.Response,
) (io.Reader, error) {
	if demo.Kind == fixedresponses.KindTools {
		return s.toolDemoTurn(ctx, chat, userMessage, demo)
	}

	return s.persistedTextDemoTurn(ctx, chat, userMessage, demo.Text)
}

func (s *Service) persistedTextDemoTurn(
	ctx context.Context,
	chat *models.Chat,
	userMessage, text string,
) (io.Reader, error) {
	unlockTurn, err := lockChatTurn(ctx, chat.ID)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "lock chat turn")
	}

	defer releaseUntransferredChatTurn(&unlockTurn)

	turnID := uuid.New()
	if err := s.savePendingUserMessage(
		ctx,
		chat.ID,
		turnID,
		userMessage,
	); err != nil {
		return nil, ctxerrors.Wrap(err, "save fixed-response user message")
	}

	run := func(rctx context.Context, pw io.Writer) error {
		return s.runDemoStream(rctx, pw, chat, text)
	}

	body := s.streamCanned(ctx, chat, text, turnID, unlockTurn, run)
	unlockTurn = nil

	return body, nil
}

func (s *Service) toolDemoTurn(
	ctx context.Context,
	chat *models.Chat,
	userMessage string,
	demo fixedresponses.Response,
) (io.Reader, error) {
	unlockTurn, err := lockChatTurn(ctx, chat.ID)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "lock chat turn")
	}

	defer releaseUntransferredChatTurn(&unlockTurn)

	turnID := uuid.New()
	if err := s.savePendingUserMessage(
		ctx,
		chat.ID,
		turnID,
		userMessage,
	); err != nil {
		return nil, ctxerrors.Wrap(err, "save demo user message")
	}

	run := func(rctx context.Context, pw io.Writer) error {
		return s.runToolDemoStream(rctx, pw, chat, demo)
	}

	body := s.streamCanned(
		ctx,
		chat,
		demo.AnswerText(),
		turnID,
		unlockTurn,
		run,
	)
	unlockTurn = nil

	return body, nil
}

// streamCanned runs a canned SSE producer in a goroutine, piping its frames to
// the response body and persisting the assistant message on completion. Used
// by durable fixed responses such as showcase catalog replies.
func (s *Service) streamCanned(
	ctx context.Context,
	chat *models.Chat,
	persistText string,
	turnID uuid.UUID,
	onDone func(),
	run func(ctx context.Context, pw io.Writer) error,
) io.Reader {
	pr, pw := io.Pipe()

	go func() {
		if onDone != nil {
			defer onDone()
		}

		defer func() { _ = pw.Close() }()

		stopCancelWatch := watchDemoPipeCancellation(ctx, pw, chat.ID)
		defer stopCancelWatch()

		defer func() {
			if r := recover(); r != nil {
				ctxscope.GetLogger(ctx).Error("demo stream panic",
					"chat_id", chat.ID,
					"panic", r,
					"reason", "demo_stream_panic",
				)
			}
		}()

		if err := run(ctx, pw); err != nil {
			ctxscope.GetLogger(ctx).Error("demo stream failed",
				"chat_id", chat.ID, "err", err, "reason", "demo_stream_failed")

			return
		}

		if err := s.persistDemoTurn(
			ctx,
			chat.ID,
			turnID,
			chat.ModelID,
			persistText,
		); err != nil {
			ctxscope.GetLogger(ctx).Warn(
				"persist demo turn failed",
				"chat_id", chat.ID,
				"err", err,
				"reason", "demo_turn_transaction_failed",
			)
		}
	}()

	return pr
}

func watchDemoPipeCancellation(
	ctx context.Context,
	pw *io.PipeWriter,
	chatID uuid.UUID,
) func() bool {
	return context.AfterFunc(ctx, func() {
		if err := pw.CloseWithError(ctx.Err()); err != nil {
			ctxscope.GetLogger(ctx).Debug(
				"canceled demo pipe already closed",
				"chat_id", chatID,
				"err", err,
				"reason", "demo_pipe_already_closed",
			)
		}
	})
}

// runDemoStream emits the SSE envelope for a canned response: preamble, the
// tokenized text block streamed one token per delta, epilogue.
func (s *Service) runDemoStream(
	ctx context.Context,
	pw io.Writer,
	chat *models.Chat,
	canned string,
) error {
	tokens, err := tiktoken.Tokenize(canned)
	if err != nil {
		return ctxerrors.Wrap(err, "tokenize demo text")
	}

	pub := essessey.NewPublisher(ctx, essesseysse.NewWriterSink(pw))
	if err := publishDemoStartStatuses(pub); err != nil {
		return err
	}

	if err := pub.SendStreamPreamble(
		newMessageID(), chat.ID.String(), chat.ModelID,
	); err != nil {
		return ctxerrors.Wrap(err, "send demo preamble")
	}

	if err := publishTurnStatus(pub, turnStatusStreaming); err != nil {
		return err
	}

	streamer := essessey.NewTextStreamer(pub, 0)
	if err := streamTokens(ctx, streamer, tokens); err != nil {
		return ctxerrors.Wrap(err, "stream demo tokens")
	}

	if err := pub.SendStreamEpilogue(
		essessey.StopReasonEndTurn, len(tokens),
	); err != nil {
		return ctxerrors.Wrap(err, "send demo epilogue")
	}

	return nil
}

// streamTokens emits each token to the streamer as its own SSE delta, paced
// so a canned demo streams progressively — like a live model turn — instead
// of landing in one burst.
func streamTokens(
	ctx context.Context,
	streamer *essessey.TextStreamer,
	tokens []string,
) error {
	for _, tok := range tokens {
		if err := streamer.Write(ctx, tok); err != nil {
			return ctxerrors.Wrap(err, "write demo token")
		}

		select {
		case <-ctx.Done():
			return ctxerrors.Wrap(ctx.Err(), "demo token delay")
		case <-time.After(demoTokenDelay):
		}
	}

	if err := streamer.Close(ctx); err != nil {
		return ctxerrors.Wrap(err, "close demo streamer")
	}

	return nil
}

// runToolDemoStream replays a scripted agentic turn as SSE blocks in order:
// thinking, text, and tool_use/tool_result pairs. Block indices increment per
// emitted block so the client's ordered block model renders text → tool → text
// interleaving. No LLM or MCP server is involved.
func (s *Service) runToolDemoStream(
	ctx context.Context,
	pw io.Writer,
	chat *models.Chat,
	demo fixedresponses.Response,
) error {
	pub := essessey.NewPublisher(ctx, essesseysse.NewWriterSink(pw))
	if err := publishDemoStartStatuses(pub); err != nil {
		return err
	}

	err := pub.SendStreamPreamble(
		newMessageID(), chat.ID.String(), chat.ModelID,
	)
	if err != nil {
		return ctxerrors.Wrap(err, "send tool demo preamble")
	}

	if err := waitDemoDelay(ctx, demo.InitialDelay); err != nil {
		return err
	}

	if err := publishTurnStatus(pub, turnStatusStreaming); err != nil {
		return err
	}

	if err := s.streamDemoSteps(ctx, pub, demo); err != nil {
		return err
	}

	err = pub.SendStreamEpilogue(essessey.StopReasonEndTurn, 0)
	if err != nil {
		return ctxerrors.Wrap(err, "send tool demo epilogue")
	}

	return nil
}

func publishDemoStartStatuses(pub *essessey.Publisher) error {
	if err := publishTurnStatus(pub, turnStatusConnecting); err != nil {
		return err
	}

	return publishTurnStatus(pub, turnStatusWaitingForFirstToken)
}

func (s *Service) streamDemoSteps(
	ctx context.Context,
	pub *essessey.Publisher,
	demo fixedresponses.Response,
) error {
	index := 0

	for stepIndex, step := range demo.Steps {
		afterTool := stepIndex > 0 &&
			demo.Steps[stepIndex-1].Kind == fixedresponses.StepTool
		if afterTool {
			if err := publishTurnStatus(
				pub,
				turnStatusWaitingForFirstToken,
			); err != nil {
				return err
			}
		}

		if err := waitDemoDelay(ctx, step.DelayBefore); err != nil {
			return err
		}

		if afterTool {
			if err := publishTurnStatus(pub, turnStatusStreaming); err != nil {
				return err
			}
		}

		next, err := s.streamDemoStep(
			ctx,
			pub,
			index,
			step,
			demo.TextChunkDelay,
		)
		if err != nil {
			return err
		}

		index = next
	}

	return nil
}

// streamDemoStep emits one scripted step at the given base block index and
// returns the next free index. A tool step consumes two indices (tool_use +
// tool_result); thinking and text consume one.
func (s *Service) streamDemoStep(
	ctx context.Context,
	pub *essessey.Publisher,
	index int,
	step fixedresponses.Step,
	chunkDelay time.Duration,
) (int, error) {
	switch step.Kind {
	case fixedresponses.StepThinking:
		thinking := essessey.NewThinkingStreamer(pub, index)
		if err := streamDemoBlock(
			ctx,
			thinking,
			step.Text,
			chunkDelay,
		); err != nil {
			return 0, ctxerrors.Wrap(err, "stream demo thinking block")
		}

		return index + 1, nil
	case fixedresponses.StepText:
		text := essessey.NewTextStreamer(pub, index)
		if err := streamDemoBlock(
			ctx,
			text,
			step.Text,
			chunkDelay,
		); err != nil {
			return 0, ctxerrors.Wrap(err, "stream demo text block")
		}

		return index + 1, nil
	case fixedresponses.StepTool:
		if err := publishTurnStatus(pub, turnStatusRunningTool); err != nil {
			return 0, err
		}

		return streamDemoToolStep(ctx, pub, index, step.Tool)
	default:
		return index, nil
	}
}

// streamDemoToolStep emits a tool_use block, pauses so the CALLING state shows,
// then emits the tool_result block. Returns the next free block index.
func streamDemoToolStep(
	ctx context.Context,
	pub *essessey.Publisher,
	index int,
	tool *fixedresponses.ToolStep,
) (int, error) {
	err := pub.SendToolUseBlock(
		index, tool.ToolUseID, tool.Name, tool.ArgsJSON,
	)
	if err != nil {
		return 0, ctxerrors.Wrap(err, "send demo tool_use block")
	}

	if err := waitDemoDelay(ctx, demoToolResultDelay(tool)); err != nil {
		return 0, err
	}

	err = pub.SendToolResultBlock(
		index+1, tool.ToolUseID, tool.ResultText, tool.IsError,
	)
	if err != nil {
		return 0, ctxerrors.Wrap(err, "send demo tool_result block")
	}

	return index + demoToolBlocks, nil
}

func demoToolResultDelay(tool *fixedresponses.ToolStep) time.Duration {
	if tool.ResultDelay > 0 {
		return tool.ResultDelay
	}

	return demoToolLatency
}

// streamDemoBlock chunks text through a streamer (rune-safe) so the client sees
// multiple deltas like a real turn, then closes the block.
func streamDemoBlock(
	ctx context.Context,
	streamer *essessey.TextStreamer,
	text string,
	chunkDelay time.Duration,
) error {
	if chunkDelay == 0 {
		chunkDelay = demoChunkDelay
	}

	for _, chunk := range chunkRunes(text, demoChunkRunes) {
		if err := streamer.Write(ctx, chunk); err != nil {
			return ctxerrors.Wrap(err, "write demo chunk")
		}

		// Pace the deltas so the demo streams progressively, not in one burst.
		if err := waitDemoDelay(ctx, chunkDelay); err != nil {
			return err
		}
	}

	if err := streamer.Close(ctx); err != nil {
		return ctxerrors.Wrap(err, "close demo streamer")
	}

	return nil
}

func waitDemoDelay(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}

	select {
	case <-ctx.Done():
		return ctxerrors.Wrap(ctx.Err(), "demo stream delay")
	case <-time.After(delay):
	}

	return nil
}

func (s *Service) persistDemoTurn(
	ctx context.Context,
	chatID, turnID uuid.UUID,
	modelID string,
	canned string,
) error {
	pctx, cancel := context.WithTimeout(
		context.WithoutCancel(ctx), persistTimeout,
	)
	defer cancel()

	row, err := toMessageRow(chatID, elelem.Message{
		Role:    elelem.RoleAssistant,
		Content: elelem.Text(canned),
	})
	if err != nil {
		return ctxerrors.Wrap(err, "convert demo assistant message")
	}

	turnComplete := true
	row.TurnID = turnID
	row.TurnComplete = &turnComplete
	row.ModelID = modelID

	if err := s.persistCompletedTurn(
		pctx,
		chatID,
		turnID,
		modelID,
		[]*models.Message{row},
	); err != nil {
		return ctxerrors.Wrap(err, "persist completed demo turn")
	}

	return nil
}

// chunkRunes splits s into rune-safe slices of at most size runes each, so a
// multibyte character is never cut across two SSE deltas.
func chunkRunes(s string, size int) []string {
	runes := []rune(s)
	if len(runes) == 0 {
		return nil
	}

	out := make([]string, 0, (len(runes)+size-1)/size)
	for start := 0; start < len(runes); start += size {
		end := min(start+size, len(runes))
		out = append(out, string(runes[start:end]))
	}

	return out
}

// genParams builds the LLM generation knobs from a chat's stored settings,
// widening the max-output-tokens cap to the client's int64 field. An unset
// setting stays a nil pointer / empty string so the client omits it.
func genParams(chat *models.Chat) elelem.GenerationParams {
	var maxTokens *int64

	if chat.MaxOutputTokens != nil {
		v := int64(*chat.MaxOutputTokens)
		maxTokens = &v
	}

	return elelem.GenerationParams{
		Temperature:     chat.Temperature,
		TopP:            chat.TopP,
		ReasoningEffort: chat.ReasoningEffort,
		MaxOutputTokens: maxTokens,
	}
}

// persistTurn writes the completed turn atomically and bumps the chat's
// activity time. Detached from the request ctx + best-effort: a disconnect
// mustn't lose the exchange, and a failed transaction only warns.
func (s *Service) persistTurn(
	ctx context.Context,
	chatID uuid.UUID,
	turnID uuid.UUID,
	result *chatcore.Result,
) {
	pctx, cancel := context.WithTimeout(
		context.WithoutCancel(ctx), persistTimeout,
	)
	defer cancel()

	logger := ctxscope.GetLogger(pctx)

	rows, err := turnRows(chatID, result.Messages)
	if err != nil {
		logger.Warn(
			"persist turn failed",
			"chat_id", chatID,
			"err", err,
			"reason", "message_marshal_failed",
		)

		return
	}

	turnComplete := true

	for _, row := range rows {
		row.TurnID = turnID
		row.TurnComplete = &turnComplete
		row.ModelID = result.ModelID
	}

	err = s.persistCompletedTurn(pctx, chatID, turnID, result.ModelID, rows)
	if err != nil {
		logger.Warn(
			"persist turn failed",
			"chat_id", chatID,
			"err", err,
			"reason", "turn_transaction_failed",
		)
	}
}

func (s *Service) persistCompletedTurn(
	ctx context.Context,
	chatID, turnID uuid.UUID,
	modelID string,
	rows []*models.Message,
) error {
	if err := s.query.Transaction(func(tx *repositories.Query) error {
		checkpointRepo := tx.Message
		if _, err := checkpointRepo.WithContext(ctx).
			Where(
				checkpointRepo.ChatID.Eq(chatID),
				checkpointRepo.TurnID.Eq(turnID),
				checkpointRepo.Role.Eq(models.MessageRoleAssistant),
				checkpointRepo.TurnComplete.Is(false),
			).
			Delete(); err != nil {
			return ctxerrors.Wrap(err, "delete assistant checkpoint")
		}

		if len(rows) > 0 {
			if err := tx.Message.WithContext(ctx).Create(rows...); err != nil {
				return ctxerrors.Wrap(err, "create turn messages")
			}
		}

		if err := completePendingUserTurn(
			ctx,
			tx,
			chatID,
			turnID,
			modelID,
		); err != nil {
			return ctxerrors.Wrap(err, "complete pending user turn")
		}

		repo := tx.Chat
		if _, err := repo.WithContext(ctx).
			Where(repo.ID.Eq(chatID)).
			Update(repo.ModelID, modelID); err != nil {
			return ctxerrors.Wrap(err, "record completed turn model")
		}

		if _, err := repo.WithContext(ctx).
			Where(repo.ID.Eq(chatID)).
			Update(repo.UpdatedAt, time.Now()); err != nil {
			return ctxerrors.Wrap(err, "touch chat")
		}

		return nil
	}); err != nil {
		return ctxerrors.Wrap(err, "persist completed turn transaction")
	}

	return nil
}

func completePendingUserTurn(
	ctx context.Context,
	query *repositories.Query,
	chatID, turnID uuid.UUID,
	modelID string,
) error {
	repo := query.Message

	result, err := repo.WithContext(ctx).
		Where(
			repo.ChatID.Eq(chatID),
			repo.TurnID.Eq(turnID),
			repo.Role.Eq(models.MessageRoleUser),
			repo.TurnComplete.Is(false),
		).
		Update(repo.TurnComplete, true)
	if err != nil {
		return ctxerrors.Wrap(err, "update pending user message")
	}

	if result.RowsAffected != 1 {
		return ctxerrors.New(fmt.Sprintf(
			"expected one pending user message, updated %d",
			result.RowsAffected,
		))
	}

	if _, err := repo.WithContext(ctx).
		Where(
			repo.ChatID.Eq(chatID),
			repo.TurnID.Eq(turnID),
			repo.Role.Eq(models.MessageRoleUser),
		).
		Update(repo.ModelID, modelID); err != nil {
		return ctxerrors.Wrap(err, "record pending user message model")
	}

	return nil
}

func turnRows(
	chatID uuid.UUID,
	messages []elelem.Message,
) ([]*models.Message, error) {
	rows := make([]*models.Message, 0, len(messages))

	for index, message := range messages {
		if !belongsInStoredHistory(messages, index) {
			continue
		}

		row, err := toMessageRow(chatID, message)
		if err != nil {
			return nil, ctxerrors.Wrap(err, "convert turn message")
		}

		row.IsInjection = message.Origin == elelem.MessageOriginInjection

		rows = append(rows, row)
	}

	return rows, nil
}

// belongsInStoredHistory decides what survives the turn.
//
// Injections are per-run scaffolding: a tool telling the model what to do with
// what it just returned. The injector re-creates them on the next run, so
// storing them would replay stale instructions forever — which is why only
// MessageOriginTurn is normally kept.
//
// The exception is an injection the run ENDED on. Nothing consumed it, and
// because injections exist only inside the run that made them, dropping it here
// loses it for good: the turn is stored as though the instruction was never
// issued. Ordering makes this exact — injections are appended after every tool
// result, so one can only be last if the run stopped before the assistant
// answered.
func belongsInStoredHistory(messages []elelem.Message, index int) bool {
	if messages[index].Origin == elelem.MessageOriginTurn {
		return true
	}

	return messages[index].Origin == elelem.MessageOriginInjection &&
		index == len(messages)-1
}

func (s *Service) savePendingUserMessage(
	ctx context.Context,
	chatID, turnID uuid.UUID,
	content string,
) error {
	pctx, cancel := context.WithTimeout(
		context.WithoutCancel(ctx), persistTimeout,
	)
	defer cancel()

	turnComplete := false
	row := &models.Message{
		ChatID:       chatID,
		TurnID:       turnID,
		TurnComplete: &turnComplete,
		Role:         models.MessageRoleUser,
		Content:      content,
	}

	if err := s.query.Transaction(func(tx *repositories.Query) error {
		if err := tx.Message.WithContext(pctx).Create(row); err != nil {
			return ctxerrors.Wrap(err, "create pending user message")
		}

		repo := tx.Chat
		if _, err := repo.WithContext(pctx).
			Where(repo.ID.Eq(chatID)).
			Update(repo.UpdatedAt, time.Now()); err != nil {
			return ctxerrors.Wrap(err, "touch chat for pending user message")
		}

		return nil
	}); err != nil {
		return ctxerrors.Wrap(err, "save pending user message transaction")
	}

	return nil
}

// loadHistory reconstructs the chat's durable user, assistant, and tool
// transcript for the next turn. Persisted system and unknown roles are not
// replayed. selectPromptHistory applies the outbound context budget after this
// complete transcript is reconstructed.
func (s *Service) loadHistory(
	ctx context.Context,
	chat *models.Chat,
) ([]elelem.Message, error) {
	repo := s.query.Message

	rows, err := repo.WithContext(ctx).
		Where(
			repo.ChatID.Eq(chat.ID),
			repo.TurnComplete.Is(true),
		).
		Order(repo.Position).
		Find()
	if err != nil {
		return nil, ctxerrors.Wrap(err, "load messages")
	}

	logger := ctxscope.GetLogger(ctx)
	out := make([]elelem.Message, 0, len(rows))

	for _, turn := range groupHistoryRows(rows) {
		messages, err := historyMessagesFromRows(turn.rows)
		if err == nil {
			err = validateToolHistory(messages)
		}

		if err != nil {
			logger.Warn(
				"skip invalid history turn",
				"chat_id", chat.ID,
				"turn_id", turn.id,
				"err", err,
				"reason", "invalid_persisted_history_turn",
			)

			continue
		}

		out = append(out, messages...)
	}

	return out, nil
}

type persistedHistoryTurn struct {
	id   uuid.UUID
	rows []*models.Message
}

// storedToolCall preserves the existing database JSON shape while elelem uses
// json.RawMessage at runtime.
type storedToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

func groupHistoryRows(rows []*models.Message) []persistedHistoryTurn {
	turns := make([]persistedHistoryTurn, 0)
	turnIndexes := make(map[uuid.UUID]int)

	for _, row := range rows {
		turnID := uuid.Nil
		if row != nil {
			turnID = row.TurnID
		}

		index, exists := turnIndexes[turnID]
		if !exists {
			index = len(turns)
			turnIndexes[turnID] = index
			turns = append(turns, persistedHistoryTurn{id: turnID})
		}

		turns[index].rows = append(turns[index].rows, row)
	}

	return turns
}

func historyMessagesFromRows(
	rows []*models.Message,
) ([]elelem.Message, error) {
	messages := make([]elelem.Message, 0, len(rows))

	for _, row := range rows {
		message, err := messageFromRow(row)
		if err != nil {
			return nil, ctxerrors.Wrap(err, "reconstruct history message")
		}

		if message.Role == "" {
			continue
		}

		messages = append(messages, message)
	}

	if len(messages) == 0 {
		return nil, ctxerrors.New("history turn has no replayable messages")
	}

	return messages, nil
}

func messageFromRow(row *models.Message) (elelem.Message, error) {
	if row == nil {
		return elelem.Message{}, ctxerrors.New("message row is nil")
	}

	if !isDurableMessage(row) {
		return elelem.Message{}, nil
	}

	// The stored column is text: every message chatz produces is text, so this
	// is a widening, not a lossy read. Accepting image or document input would
	// mean storing the parts rather than a string — see toMessageRow.
	//
	// An empty column stays EMPTY content rather than becoming a text part
	// holding "". An assistant turn that only called tools has no text, and
	// manufacturing a blank part would make it a message that carries one —
	// which round-trips as unequal and would have to be filtered back out by
	// everything downstream that asks what the message contains.
	message := elelem.Message{
		Role:              row.Role,
		ToolCallID:        row.ToolCallID,
		ToolResultIsError: row.IsError,
	}

	if row.Content != "" {
		message.Content = elelem.Text(row.Content)
	}

	// Carry the injection marker back onto the message so elelem — which owns
	// the injection mechanism — applies its own rule about replaying one.
	if row.IsInjection {
		message.Origin = elelem.MessageOriginInjection
	}

	reasoning, err := storedProviderReasoning(row)
	if err != nil {
		return elelem.Message{}, err
	}

	message.ProviderReasoning = reasoning

	if len(row.ToolCalls) > 0 {
		calls, err := storedToolCallsFromJSON(row.ToolCalls)
		if err != nil {
			return elelem.Message{}, ctxerrors.Wrap(
				err,
				"unmarshal tool calls",
			)
		}

		message.ToolCalls = calls
	}

	if shouldSkipEmptyAssistant(message) {
		return elelem.Message{}, nil
	}

	return message, nil
}

// storedProviderReasoning returns the row's opaque provider reasoning, or nil
// when there is none to replay.
//
// Invalid JSON is an error even for a role that would not carry the field: a
// corrupt blob means the row was written by something that did not understand
// the column, and replaying the rest of that row is not obviously safe.
// Non-assistant rows return nil because the field is meaningless off an
// assistant turn — the provider rejects it there.
func storedProviderReasoning(
	row *models.Message,
) (json.RawMessage, error) {
	if len(row.ProviderReasoning) == 0 {
		return nil, nil
	}

	if !json.Valid(row.ProviderReasoning) {
		return nil, ctxerrors.New("stored provider reasoning is invalid JSON")
	}

	if row.Role != models.MessageRoleAssistant {
		return nil, nil
	}

	return append(json.RawMessage(nil), row.ProviderReasoning...), nil
}

// isDurableMessage reports whether a stored row may re-enter a prompt.
//
// The system role is deliberately NOT replayable on its own. A stored system
// message is a prompt-injection vector: anything that can write one row gets it
// replayed to the model as a system instruction on every later turn. Real
// system prompts are built per request and never read back from the database,
// so a bare stored system row has no legitimate source.
//
// The single exception is a row this service wrote itself for a tool injection,
// which carries IsInjection. It is loaded only so elelem — which owns the
// injection mechanism — applies its own replay rule to it.
func isDurableMessage(row *models.Message) bool {
	if row.Role == models.MessageRoleSystem {
		return row.IsInjection
	}

	return row.Role == models.MessageRoleUser ||
		row.Role == models.MessageRoleAssistant ||
		row.Role == models.MessageRoleTool
}

func shouldSkipEmptyAssistant(message elelem.Message) bool {
	return message.Role == elelem.RoleAssistant &&
		message.Text() == "" &&
		len(message.ToolCalls) == 0 &&
		len(message.ProviderReasoning) == 0 &&
		message.ToolCallID == "" &&
		!message.ToolResultIsError
}

func validateToolHistory(messages []elelem.Message) error {
	validator := toolHistoryValidator{}

	for i, message := range messages {
		if err := validator.validateMessage(i, message); err != nil {
			return ctxerrors.Wrap(err, "validate history message")
		}
	}

	if len(validator.pending) > 0 {
		return ctxerrors.New("message history ends before all tool results")
	}

	return nil
}

type toolHistoryValidator struct {
	pending map[string]struct{}
}

func (v *toolHistoryValidator) validateMessage(
	index int,
	message elelem.Message,
) error {
	if len(v.pending) > 0 {
		return v.validateToolResult(index, message)
	}

	switch message.Role {
	case elelem.RoleUser:
		return validateUserHistoryMessage(index, message)
	case elelem.RoleAssistant:
		return v.validateAssistantMessage(index, message)
	case elelem.RoleTool:
		return historyValidationError("orphan tool result at message %d", index)
	default:
		return historyValidationError(
			"unsupported role %q at message %d",
			message.Role,
			index,
		)
	}
}

func validateUserHistoryMessage(index int, message elelem.Message) error {
	if message.ToolCallID == "" &&
		len(message.ToolCalls) == 0 &&
		!message.ToolResultIsError {
		return nil
	}

	return historyValidationError(
		"user message %d contains tool metadata",
		index,
	)
}

func (v *toolHistoryValidator) validateAssistantMessage(
	index int,
	message elelem.Message,
) error {
	if message.ToolCallID != "" || message.ToolResultIsError {
		return historyValidationError(
			"assistant message %d contains tool-result metadata",
			index,
		)
	}

	if len(message.ToolCalls) == 0 {
		return nil
	}

	v.pending = make(map[string]struct{}, len(message.ToolCalls))
	for _, call := range message.ToolCalls {
		if err := v.addToolCall(index, call); err != nil {
			return ctxerrors.Wrap(err, "validate assistant tool call")
		}
	}

	return nil
}

func (v *toolHistoryValidator) addToolCall(
	index int,
	call elelem.ToolCall,
) error {
	if call.ID == "" {
		return historyValidationError(
			"assistant message %d contains a tool call with no ID",
			index,
		)
	}

	if call.Name == "" {
		return historyValidationError(
			"assistant message %d contains tool call %q with no name",
			index,
			call.ID,
		)
	}

	if _, exists := v.pending[call.ID]; exists {
		return historyValidationError(
			"assistant message %d contains duplicate tool call ID %q",
			index,
			call.ID,
		)
	}

	v.pending[call.ID] = struct{}{}

	return nil
}

func (v *toolHistoryValidator) validateToolResult(
	index int,
	message elelem.Message,
) error {
	if message.Role != elelem.RoleTool {
		return historyValidationError(
			"message %d interrupts pending tool results",
			index,
		)
	}

	if message.ToolCallID == "" {
		return historyValidationError(
			"tool result at message %d has no call ID",
			index,
		)
	}

	if _, ok := v.pending[message.ToolCallID]; !ok {
		return historyValidationError(
			"tool result at message %d has unknown or duplicate call ID %q",
			index,
			message.ToolCallID,
		)
	}

	if len(message.ToolCalls) > 0 {
		return historyValidationError(
			"tool result at message %d contains tool calls",
			index,
		)
	}

	delete(v.pending, message.ToolCallID)

	return nil
}

func historyValidationError(format string, args ...any) error {
	return ctxerrors.New(fmt.Sprintf(format, args...))
}

func toMessageRow(
	chatID uuid.UUID,
	m elelem.Message,
) (*models.Message, error) {
	// Content is stored as text because every message chatz produces is text.
	// The day a user message can carry an image, this column has to hold the
	// parts instead — Text() would silently drop the attachment.
	row := &models.Message{
		ChatID:     chatID,
		Role:       m.Role,
		Content:    m.Text(),
		Reasoning:  m.Reasoning,
		ToolCallID: m.ToolCallID,
		IsError:    m.ToolResultIsError,
	}

	if m.Role == elelem.RoleAssistant && len(m.ProviderReasoning) > 0 {
		if !json.Valid(m.ProviderReasoning) {
			return nil, ctxerrors.New("provider reasoning is invalid JSON")
		}

		row.ProviderReasoning = append(
			datatypes.JSON(nil),
			m.ProviderReasoning...,
		)
	}

	if len(m.ToolCalls) > 0 {
		raw, err := storedToolCallsJSON(m.ToolCalls)
		if err != nil {
			return nil, ctxerrors.Wrap(err, "marshal tool calls")
		}

		row.ToolCalls = raw
	}

	return row, nil
}

func storedToolCallsFromJSON(raw []byte) ([]elelem.ToolCall, error) {
	var stored []storedToolCall
	if err := json.Unmarshal(raw, &stored); err != nil {
		return nil, ctxerrors.Wrap(err, "unmarshal stored tool calls")
	}

	calls := make([]elelem.ToolCall, 0, len(stored))
	for _, call := range stored {
		arguments := json.RawMessage(call.Arguments)
		if !json.Valid(arguments) {
			return nil, ctxerrors.New(
				"stored tool call arguments are invalid JSON",
			)
		}

		calls = append(calls, elelem.ToolCall{
			ID:        call.ID,
			Name:      call.Name,
			Arguments: arguments,
		})
	}

	return calls, nil
}

func storedToolCallsJSON(calls []elelem.ToolCall) ([]byte, error) {
	stored := make([]storedToolCall, 0, len(calls))
	for _, call := range calls {
		if !json.Valid(call.Arguments) {
			return nil, ctxerrors.New("tool call arguments are invalid JSON")
		}

		stored = append(stored, storedToolCall{
			ID:        call.ID,
			Name:      call.Name,
			Arguments: string(call.Arguments),
		})
	}

	raw, err := json.Marshal(stored)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "marshal stored tool calls")
	}

	return raw, nil
}

func newMessageID() string {
	return "msg_" + strings.ReplaceAll(uuid.NewString(), "-", "")
}

// chatTitle derives a chat title from its first message, truncated by rune.
func chatTitle(message string) string {
	runes := []rune(strings.TrimSpace(message))
	if len(runes) > maxTitleRunes {
		return string(runes[:maxTitleRunes])
	}

	return string(runes)
}
