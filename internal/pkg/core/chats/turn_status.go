package chats

import (
	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/essessey"
)

const (
	streamStatusEvent = "chat_status"

	turnStatusConnecting           = "connecting"
	turnStatusWaitingForFirstToken = "waiting_first_token"
	turnStatusStreaming            = "streaming"
	turnStatusRunningTool          = "running_tool"
	turnStatusRetrying             = "retrying"
)

type turnStatusData struct {
	Type   string `json:"type"`
	Status string `json:"status"`
}

func publishTurnStatus(pub *essessey.Publisher, status string) error {
	if err := pub.Publish(streamStatusEvent, turnStatusData{
		Type:   streamStatusEvent,
		Status: status,
	}); err != nil {
		return ctxerrors.Wrap(err, "publish chat turn status")
	}

	return nil
}
