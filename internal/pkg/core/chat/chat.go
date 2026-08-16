// Package chat adapts elelem's provider-neutral agent loop to chatz SSE.
package chat

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/psyb0t/chatz/internal/pkg/heartbeat"
	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/ctxscope"
	"github.com/psyb0t/elelem"
	"github.com/psyb0t/essessey"
	"github.com/psyb0t/essessey/elelemstream"
)

const defaultMaxRounds = 12

type Deps struct {
	Client    *elelem.Client
	Tools     ToolExecutor
	Publisher *essessey.Publisher
}

type Request struct {
	MessageID        string
	ConversationID   string
	Model            elelem.Model
	Prompt           elelem.Prompt
	MaxRounds        int
	MaxContextTokens int
	Params           elelem.GenerationParams
	OnStreamStart    func()
	OnAssistantDelta func(context.Context, elelem.Delta)
	OnFirstDelta     func(context.Context) error
	OnRoundStart     func(context.Context, *elelem.RoundEvent) error
	OnToolCallStart  func(context.Context, elelem.ToolCallEvent) error
}

type Result struct {
	MessageID string
	ModelID   string
	FinalText string
	Usage     elelem.Usage
	Rounds    int
	Messages  []elelem.Message
}

type streamRunError struct {
	err           error
	streamStarted bool
}

func (e *streamRunError) Error() string {
	return e.err.Error()
}

func (e *streamRunError) Unwrap() error {
	return e.err
}

// StreamStarted reports whether Run emitted any stream event before err. A
// caller may only retry with another model when this is false: once the stream
// begins, a client may have displayed output and the agent may have reached a
// tool-capable round.
func StreamStarted(err error) bool {
	var runErr *streamRunError

	return errors.As(err, &runErr) && runErr.streamStarted
}

func Run(ctx context.Context, deps Deps, req Request) (*Result, error) {
	maxRounds := req.MaxRounds
	if maxRounds <= 0 {
		maxRounds = defaultMaxRounds
	}

	state := newAdapterState(ctx, deps.Publisher, req, maxRounds)
	elelemRequest := newElelemRequest(deps, req, state, maxRounds)

	response, err := elelemRequest.Run(ctx)

	state.stopHeartbeat()

	if err != nil {
		return nil, &streamRunError{
			err:           ctxerrors.Wrap(err, "run elelem chat turn"),
			streamStarted: state.streamStarted,
		}
	}

	if err := state.startStream(ctx); err != nil {
		return nil, &streamRunError{
			err:           ctxerrors.Wrap(err, "start chat stream"),
			streamStarted: state.streamStarted,
		}
	}

	stopReason := mapStopReason(response.FinishReason, response.ToolCalls)
	if err := deps.Publisher.SendStreamEpilogue(
		stopReason,
		int(response.Usage.Completion),
	); err != nil {
		return nil, ctxerrors.Wrap(err, "send stream epilogue")
	}

	ctxscope.GetLogger(ctx).Info(
		"chat turn complete",
		"message_id", req.MessageID,
		"rounds", state.blocks.Rounds(),
		"total_tokens", response.Usage.Total,
		"stop_reason", stopReason,
	)

	return &Result{
		MessageID: req.MessageID,
		ModelID:   req.Model.ID,
		FinalText: response.Text,
		Usage:     response.Usage,
		Rounds:    state.blocks.Rounds(),
		Messages:  response.Messages,
	}, nil
}

// newElelemRequest registers chatz's own hooks FIRST and binds the block
// adapter after, so that on every event chatz runs before the block is
// emitted — the heartbeat has to be touched whether or not the block write
// then fails, and a round's log line should precede its blocks.
//
// Both sets run: elelem's On* setters append to a chain rather than replace.
func newElelemRequest(
	deps Deps,
	req Request,
	state *adapterState,
	maxRounds int,
) *elelem.Request {
	request := state.blocks.Bind(
		elelem.NewRequest(deps.Client).
			WithModel(req.Model).
			WithPrompt(req.Prompt).
			WithGenerationParams(req.Params).
			WithMaxRounds(maxRounds).
			WithMaxConcurrentTools(maxConcurrentTools).
			WithForceFinalAnswer(true).
			WithAutoToolCalls().
			WithTranscriptRepair().
			WithToolProvider(toolProvider(deps.Tools)).
			OnRoundStart(state.onRoundStart).
			OnDelta(state.onDelta).
			OnAssistantMessage(state.onAssistantMessage).
			OnToolCallStart(state.onToolCallStart).
			OnRoundEnd(state.onRoundEnd),
	).
		OnError(state.onError)

	if req.MaxContextTokens > 0 {
		request.WithMaxContextTokens(req.MaxContextTokens)
	}

	return request
}

// adapterState carries the concerns that are chatz's rather than the wire
// protocol's: the stall heartbeat, the per-round log lines, and the caller's
// own round hook. Everything to do with content-block indices belongs to
// blocks.
type adapterState struct {
	blocks             *elelemstream.Adapter
	publisher          *essessey.Publisher
	logger             *slog.Logger
	messageID          string
	conversationID     string
	modelID            string
	maxRounds          int
	roundStart         time.Time
	heartbeat          *heartbeat.H
	streamStarted      bool
	roundHasDelta      bool
	streamStartHook    func()
	assistantDeltaHook func(context.Context, elelem.Delta)
	firstDeltaHook     func(context.Context) error
	roundStartHook     func(context.Context, *elelem.RoundEvent) error
	toolCallStartHook  func(context.Context, elelem.ToolCallEvent) error
}

func newAdapterState(
	ctx context.Context,
	publisher *essessey.Publisher,
	req Request,
	maxRounds int,
) *adapterState {
	return &adapterState{
		blocks:             elelemstream.New(publisher),
		publisher:          publisher,
		logger:             ctxscope.GetLogger(ctx),
		messageID:          req.MessageID,
		conversationID:     req.ConversationID,
		modelID:            req.Model.ID,
		maxRounds:          maxRounds,
		streamStartHook:    req.OnStreamStart,
		assistantDeltaHook: req.OnAssistantDelta,
		firstDeltaHook:     req.OnFirstDelta,
		roundStartHook:     req.OnRoundStart,
		toolCallStartHook:  req.OnToolCallStart,
	}
}

func (state *adapterState) startStream(_ context.Context) error {
	if state.streamStarted {
		return nil
	}

	if err := state.publisher.SendStreamPreamble(
		state.messageID,
		state.conversationID,
		state.modelID,
	); err != nil {
		return ctxerrors.Wrap(err, "send stream preamble")
	}

	state.streamStarted = true
	if state.streamStartHook != nil {
		state.streamStartHook()
	}

	return nil
}

func (state *adapterState) onRoundStart(
	ctx context.Context,
	event *elelem.RoundEvent,
) error {
	state.roundStart = time.Now()
	state.roundHasDelta = false
	state.stopHeartbeat()
	state.heartbeat = heartbeat.Start(
		ctx,
		"still waiting on LLM stream",
		"model", state.modelID,
	)
	state.logger.Info(
		"round started",
		"message_id", state.messageID,
		"round", event.Round+1,
		"max_rounds", state.maxRounds,
		"without_tools", len(event.Tools) == 0,
	)

	if state.roundStartHook != nil {
		if err := state.roundStartHook(ctx, event); err != nil {
			return ctxerrors.Wrap(err, "chat on round start")
		}
	}

	return nil
}

func (state *adapterState) onDelta(
	ctx context.Context,
	delta elelem.Delta,
) error {
	if err := state.startStream(ctx); err != nil {
		return ctxerrors.Wrap(err, "start stream on delta")
	}

	if !state.roundHasDelta {
		state.roundHasDelta = true
		if state.firstDeltaHook != nil {
			if err := state.firstDeltaHook(ctx); err != nil {
				return ctxerrors.Wrap(err, "chat on first delta")
			}
		}
	}

	if state.heartbeat != nil {
		state.heartbeat.Touch()
	}

	if state.assistantDeltaHook != nil &&
		(delta.Text != "" || delta.Reasoning != "") {
		state.assistantDeltaHook(ctx, delta)
	}

	return nil
}

func (state *adapterState) onAssistantMessage(
	ctx context.Context,
	_ elelem.Message,
) error {
	if err := state.startStream(ctx); err != nil {
		return ctxerrors.Wrap(err, "start stream on assistant message")
	}

	state.stopHeartbeat()

	return nil
}

func (state *adapterState) onRoundEnd(
	_ context.Context,
	event *elelem.RoundEvent,
) error {
	roundStopReason := mapStopReason(
		event.Usage.FinishReason,
		toolCallsWithLength(event.ToolCalls),
	)
	state.logger.Info(
		"round complete",
		"message_id", state.messageID,
		"round", event.Round+1,
		"duration_ms", time.Since(state.roundStart).Milliseconds(),
		"tool_calls", event.ToolCalls,
		"finish_reason", event.Usage.FinishReason,
		"mapped_stop_reason", roundStopReason,
	)

	return nil
}

func (state *adapterState) onToolCallStart(
	ctx context.Context,
	event elelem.ToolCallEvent,
) error {
	if state.toolCallStartHook == nil {
		return nil
	}

	if err := state.toolCallStartHook(ctx, event); err != nil {
		return ctxerrors.Wrap(err, "chat on tool call start")
	}

	return nil
}

func (state *adapterState) onError(_ context.Context, _ error) error {
	state.stopHeartbeat()

	return nil
}

func (state *adapterState) stopHeartbeat() {
	if state.heartbeat == nil {
		return
	}

	state.heartbeat.Stop()
	state.heartbeat = nil
}

func mapStopReason(
	finishReason elelem.FinishReason,
	toolCalls []elelem.ToolCall,
) essessey.StopReason {
	if finishReason.IsTruncated() {
		return essessey.StopReasonMaxTokens
	}

	if len(toolCalls) > 0 {
		return essessey.StopReasonToolUse
	}

	return essessey.StopReasonEndTurn
}

func toolCallsWithLength(length int) []elelem.ToolCall {
	if length <= 0 {
		return nil
	}

	return make([]elelem.ToolCall, length)
}
