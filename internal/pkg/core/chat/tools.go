package chat

import (
	"context"
	"encoding/json"
	"runtime/debug"

	"github.com/psyb0t/chatz/internal/pkg/mcp"
	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/ctxscope"
	"github.com/psyb0t/elelem"
)

const (
	maxConcurrentTools = 8
	toolInjectionKey   = "system_message_injection"
)

type ToolExecutor interface {
	Tools(ctx context.Context) []mcp.Tool
	Call(
		ctx context.Context,
		qualifiedName string,
		args map[string]any,
	) (*mcp.ToolResult, error)
}

type toolProviderFunc func(context.Context) (*elelem.ToolSet, error)

type toolEnvelope struct {
	Result                 json.RawMessage `json:"result"`
	Error                  string          `json:"error"`
	SystemMessageInjection string          `json:"systemMessageInjection"`
}

func toolProvider(executor ToolExecutor) toolProviderFunc {
	return func(ctx context.Context) (*elelem.ToolSet, error) {
		if executor == nil {
			return elelem.NewToolSet(), nil
		}

		set := elelem.NewToolSet()
		for _, tool := range executor.Tools(ctx) {
			set.Add(toElelemTool(executor, tool))
		}

		return set, nil
	}
}

func toElelemTool(executor ToolExecutor, tool mcp.Tool) elelem.Tool {
	var schema json.RawMessage

	if tool.InputSchema != nil {
		if raw, err := json.Marshal(tool.InputSchema); err == nil {
			schema = raw
		}
	}

	result := elelem.Tool{
		Name:            tool.QualifiedName(),
		Description:     tool.Description,
		ArgumentsSchema: schema,
	}
	result.Handler = func(
		ctx context.Context,
		input elelem.ToolInput,
	) (elelem.ToolResult, error) {
		return runMCPTool(ctx, executor, input)
	}
	result.OnSuccessMessageInjector = toolMessageInjector
	result.OnErrorMessageInjector = toolMessageInjector

	return result
}

//nolint:nonamedreturns // Panic recovery replaces the named result safely.
func runMCPTool(
	ctx context.Context,
	executor ToolExecutor,
	input elelem.ToolInput,
) (out elelem.ToolResult, retErr error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			ctxscope.GetLogger(ctx).Error(
				"tool execution panic",
				"tool", input.Name,
				"panic", recovered,
				"stack", string(debug.Stack()),
			)

			out = elelem.ToolResult{
				Content: `{"error":"tool panicked"}`,
				IsError: true,
			}
			retErr = nil
		}
	}()

	args, err := decodeArgs(input.Arguments)
	if err != nil {
		ctxscope.GetLogger(ctx).Warn(
			"tool args decode failed",
			"tool", input.Name,
			"err", err,
			"reason", "bad_tool_args",
		)

		return elelem.ToolResult{Content: errorResult(err), IsError: true}, nil
	}

	return callMCPTool(ctx, executor, input.Name, args), nil
}

func callMCPTool(
	ctx context.Context,
	executor ToolExecutor,
	name string,
	args map[string]any,
) elelem.ToolResult {
	result, err := executor.Call(ctx, name, args)
	if err != nil {
		ctxscope.GetLogger(ctx).Warn(
			"tool call failed, feeding error to model",
			"tool", name,
			"err", err,
			"reason", "tool_call_failed",
		)

		return elelem.ToolResult{Content: errorResult(err), IsError: true}
	}

	if result == nil {
		return elelem.ToolResult{
			Content: `{"error":"tool returned no result"}`,
			IsError: true,
		}
	}

	content, injection := splitEnvelope(result.Text)
	metadata := map[string]any(nil)

	if injection != "" {
		metadata = map[string]any{toolInjectionKey: injection}
	}

	return elelem.ToolResult{
		Content:  content,
		IsError:  result.IsError,
		Metadata: metadata,
	}
}

func toolMessageInjector(
	_ context.Context,
	event *elelem.ToolEvent,
) (*elelem.MessageInjection, error) {
	if event == nil || event.Result == nil {
		//nolint:nilnil // No injection is a valid outcome.
		return nil, nil
	}

	injection, ok := event.Result.Metadata[toolInjectionKey].(string)
	if !ok || injection == "" {
		//nolint:nilnil // No injection is a valid outcome.
		return nil, nil
	}

	return &elelem.MessageInjection{
		Type:    elelem.RoleSystem,
		Content: injection,
	}, nil
}

func splitEnvelope(raw string) (string, string) {
	var envelope toolEnvelope
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		return raw, ""
	}

	if envelope.SystemMessageInjection == "" {
		return raw, ""
	}

	content := resultText(envelope.Result)
	if content == "" && envelope.Error != "" {
		content = envelope.Error
	}

	return content, envelope.SystemMessageInjection
}

func resultText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}

	return string(raw)
}

func decodeArgs(raw json.RawMessage) (map[string]any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}

	var args map[string]any
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, ctxerrors.Wrap(err, "unmarshal tool args")
	}

	return args, nil
}

func errorResult(err error) string {
	raw, marshalErr := json.Marshal(map[string]string{"error": err.Error()})
	if marshalErr != nil {
		return `{"error":"tool failed"}`
	}

	return string(raw)
}
