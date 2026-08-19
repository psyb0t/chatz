package chats

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/google/uuid"
	"github.com/psyb0t/chatz/internal/pkg/core/chats/fixedresponses"
	"github.com/psyb0t/chatz/internal/pkg/db/models"
	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/ctxerrors/commerr"
	"github.com/psyb0t/elelem"
	"gorm.io/gen/field"
	"gorm.io/gorm"
)

// List returns a page of the user's non-empty chats (at least one user
// message), newest activity first, plus the scoped total count. A brand-new
// chat with no user message yet — see GetOrCreateEmpty — is hidden from
// history until the user actually sends something in it.
func (s *Service) List(
	ctx context.Context,
	userID uuid.UUID,
	limit, offset int,
) ([]*models.Chat, int, error) {
	return s.ListWithOptions(
		ctx,
		userID,
		ListOptions{},
		limit,
		offset,
	)
}

// ListWithOptions returns the requested page of the user's non-empty chats.
// A blank search has no filter.
func (s *Service) ListWithOptions(
	ctx context.Context,
	userID uuid.UUID,
	options ListOptions,
	limit, offset int,
) ([]*models.Chat, int, error) {
	repo := s.query.Chat
	search := chatSearchPattern(options.Search)

	total, err := repo.WithContext(ctx).CountNonEmpty(userID, search)
	if err != nil {
		return nil, 0, ctxerrors.Wrap(err, "count chats")
	}

	list, err := repo.WithContext(ctx).ListNonEmpty(
		userID,
		search,
		limit,
		offset,
	)
	if err != nil {
		return nil, 0, ctxerrors.Wrap(err, "list chats")
	}

	return list, int(total), nil
}

// GetOrCreateEmpty returns the caller's reusable "empty chat" — the chat a
// fresh "New chat" click lands on — creating one if none exists. At most one
// empty chat is ever kept per user: repeated "New chat" clicks before typing
// anything all resolve to the SAME chat instead of piling up unused rows.
//
// A benign race exists: two concurrent calls (a double-click, two tabs) can
// both see zero existing empty chats and each create one. This is accepted
// rather than transaction-guarded — the worst case is a second, harmless
// empty row that gets reused (or ages out unused) rather than any data loss.
func (s *Service) GetOrCreateEmpty(
	ctx context.Context,
	userID uuid.UUID,
) (*models.Chat, error) {
	repo := s.query.Chat

	existing, err := repo.WithContext(ctx).FindEmptyChat(userID)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "find empty chat")
	}

	if len(existing) > 0 {
		return existing[0], nil
	}

	chat := &models.Chat{UserID: userID}
	if err := repo.WithContext(ctx).Create(chat); err != nil {
		return nil, ctxerrors.Wrap(err, "create empty chat")
	}

	return chat, nil
}

// Get returns a chat's metadata owned by userID (no messages — fetch those via
// ListMessages). Returns commerr.ErrNotFound when the chat does not
// exist or belongs to another user.
func (s *Service) Get(
	ctx context.Context,
	chatID, userID uuid.UUID,
) (*models.Chat, error) {
	return s.ownedChat(ctx, chatID, userID)
}

// ListMessages returns a page of a chat's visible messages, oldest-first,
// plus the scoped total count (indexed by chat_id). A row is visible if it
// carries anything to render — content, a reasoning trace, or tool calls;
// a row with none of those is purely internal plumbing (e.g. a completed
// round whose only output was consumed elsewhere) and is omitted from both
// the page and the total. Rows are also restricted to the three durable
// roles (user/assistant/tool) — a role="system" tool-steering injection
// should never have been persisted (persistTurn skips it going forward),
// but this filters out any that slipped in before that fix rather than
// rendering as a stray fake user bubble. An incomplete user row is visible so
// a refresh after stopping a stream retains the submitted message; an
// incomplete assistant checkpoint is also visible, while incomplete tool rows
// remain hidden. Returns commerr.ErrNotFound
// when the chat does not exist or belongs to another user.
func (s *Service) ListMessages(
	ctx context.Context,
	chatID, userID uuid.UUID,
	limit, offset int,
) ([]*models.Message, int, error) {
	chat, err := s.ownedChat(ctx, chatID, userID)
	if err != nil {
		return nil, 0, err
	}

	repo := s.query.Message

	hasContent := field.Or(
		repo.Content.Neq(""),
		repo.Reasoning.Neq(""),
		repo.ToolCalls.IsNotNull(),
	)
	durableRole := repo.Role.In(
		models.MessageRoleUser,
		models.MessageRoleAssistant,
		models.MessageRoleTool,
	)
	incompleteVisibleRole := repo.Role.In(
		models.MessageRoleUser,
		models.MessageRoleAssistant,
	)
	visibleTurn := field.Or(
		repo.TurnComplete.Is(true),
		field.And(incompleteVisibleRole, repo.TurnComplete.Is(false)),
	)

	total, err := repo.WithContext(ctx).
		Where(
			repo.ChatID.Eq(chat.ID),
			visibleTurn,
			hasContent,
			durableRole,
		).
		Count()
	if err != nil {
		return nil, 0, ctxerrors.Wrap(err, "count messages")
	}

	rows, err := repo.WithContext(ctx).
		Where(
			repo.ChatID.Eq(chat.ID),
			visibleTurn,
			hasContent,
			durableRole,
		).
		Order(repo.Position).
		Limit(limit).
		Offset(offset).
		Find()
	if err != nil {
		return nil, 0, ctxerrors.Wrap(err, "list messages")
	}

	return rows, int(total), nil
}

// Create creates a chat for the user and returns a reader that streams the
// first assistant turn (SSE). A demo command works with any model id; a real
// turn returns ErrUnknownModel when no configured upstream serves model.
func (s *Service) Create(
	ctx context.Context,
	userID uuid.UUID,
	message, model string,
) (io.Reader, error) {
	// A fixed response streams without an LLM, so resolve it before the model
	// registry. Showcase mode adds exact catalog matches only.
	demo, isDemo := s.fixedResponse(message)

	client, ok := s.clientForModel(model)
	if !ok && !isDemo {
		return nil, ErrUnknownModel
	}

	chat := &models.Chat{
		UserID:  userID,
		Title:   chatTitle(message),
		ModelID: model,
	}
	if err := s.query.Chat.WithContext(ctx).Create(chat); err != nil {
		return nil, ctxerrors.Wrap(err, "create chat")
	}

	return s.firstTurnBody(ctx, chat, message, isDemo, demo, client)
}

// Continue continues an existing chat and returns a reader that streams the
// next assistant turn (SSE). requestedModel, when non-empty, switches the chat
// to that model (persisted so a reopened chat comes back to its last-used
// model); demo turns bypass the LLM and never change the chat's model. A chat
// with no title yet (its first real message — the composer creates the chat
// via GetOrCreateEmpty before the user types anything, so this is the only
// place a title gets set) is titled from this message, mirroring what the
// standalone Create() does for its one-shot create+message flow. Returns
// commerr.ErrNotFound if the chat is missing, or ErrUnknownModel for an
// unservable real-turn model.
func (s *Service) Continue(
	ctx context.Context,
	chatID, userID uuid.UUID,
	message string,
	requestedModel *string,
) (io.Reader, error) {
	chat, err := s.ownedChat(ctx, chatID, userID)
	if err != nil {
		return nil, err
	}

	demo, isDemo := s.fixedResponse(message)
	if isDemo {
		return s.continueDemoTurn(ctx, chat, message, demo)
	}

	return s.continueRealTurn(
		ctx,
		chatID,
		userID,
		message,
		requestedModel,
	)
}

func (s *Service) fixedResponse(
	message string,
) (fixedresponses.Response, bool) {
	if !s.showcaseMode {
		return fixedresponses.Response{}, false
	}

	return fixedresponses.GetShowcase(message)
}

func (s *Service) continueDemoTurn(
	ctx context.Context,
	chat *models.Chat,
	message string,
	demo fixedresponses.Response,
) (io.Reader, error) {
	if err := s.ensureChatTitle(ctx, chat, message); err != nil {
		return nil, err
	}

	return s.nextTurnBody(ctx, chat, message, true, demo, nil, nil)
}

func (s *Service) continueRealTurn(
	ctx context.Context,
	chatID, userID uuid.UUID,
	message string,
	requestedModel *string,
) (io.Reader, error) {
	unlockTurn, err := lockChatTurn(ctx, chatID)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "lock chat turn")
	}

	defer releaseUntransferredChatTurn(&unlockTurn)

	chat, err := s.ownedChat(ctx, chatID, userID)
	if err != nil {
		return nil, err
	}

	model := chat.ModelID
	if requestedModel != nil && *requestedModel != "" {
		model = *requestedModel
	}

	client, ok := s.clientForModel(model)
	if !ok {
		return nil, ErrUnknownModel
	}

	if model != chat.ModelID {
		if err := s.setChatModel(ctx, chat.ID, model); err != nil {
			return nil, ctxerrors.Wrap(err, "switch chat model")
		}

		chat.ModelID = model
	}

	if err := s.ensureChatTitle(ctx, chat, message); err != nil {
		return nil, err
	}

	body, err := s.nextTurnBody(
		ctx,
		chat,
		message,
		false,
		fixedresponses.Response{},
		client,
		unlockTurn,
	)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "start next turn")
	}

	unlockTurn = nil

	return body, nil
}

// ensureChatTitle titles a still-untitled chat from message (the first real
// turn — see Continue's doc) and reflects the change onto chat. Split out of
// Continue to keep its branching under the complexity budget.
func (s *Service) ensureChatTitle(
	ctx context.Context,
	chat *models.Chat,
	message string,
) error {
	if chat.Title != "" {
		return nil
	}

	if err := s.setChatTitle(ctx, chat.ID, message); err != nil {
		return ctxerrors.Wrap(err, "set chat title")
	}

	chat.Title = chatTitle(message)

	return nil
}

// Rename sets a chat's title (trimmed + capped to maxTitleRunes) and returns
// the updated chat. Returns ErrEmptyTitle for a blank title and
// commerr.ErrNotFound when the chat is missing or owned by another user.
func (s *Service) Rename(
	ctx context.Context,
	chatID, userID uuid.UUID,
	title string,
) (*models.Chat, error) {
	if strings.TrimSpace(title) == "" {
		return nil, ErrEmptyTitle
	}

	chat, err := s.ownedChat(ctx, chatID, userID)
	if err != nil {
		return nil, err
	}

	newTitle := chatTitle(strings.TrimSpace(title))
	chat.Title = newTitle

	repo := s.query.Chat
	if _, err := repo.WithContext(ctx).
		Where(repo.ID.Eq(chatID)).
		Update(repo.Title, newTitle); err != nil {
		return nil, ctxerrors.Wrap(err, "rename chat")
	}

	return chat, nil
}

// Settings is the mutable per-chat generation config. A nil pointer / empty
// ReasoningEffort leaves that setting unset (provider default / no cap).
type Settings struct {
	Temperature      *float64
	TopP             *float64
	ReasoningEffort  string
	MaxOutputTokens  *int
	MaxHistoryTokens *int
}

// UpdateSettings replaces the chat's generation settings and returns the
// updated chat. Full replacement: a field left unset clears that setting.
// Returns commerr.ErrNotFound when the chat is missing or owned by
// another user.
func (s *Service) UpdateSettings(
	ctx context.Context,
	chatID, userID uuid.UUID,
	settings Settings,
) (*models.Chat, error) {
	chat, err := s.ownedChat(ctx, chatID, userID)
	if err != nil {
		return nil, err
	}

	chat.Temperature = settings.Temperature
	chat.TopP = settings.TopP
	chat.ReasoningEffort = settings.ReasoningEffort
	chat.MaxOutputTokens = settings.MaxOutputTokens
	chat.MaxHistoryTokens = settings.MaxHistoryTokens

	// Select the settings columns explicitly so nil pointers persist as NULL
	// (Updates skips zero-value fields otherwise, which would silently ignore a
	// "clear this setting" edit).
	repo := s.query.Chat
	if _, err := repo.WithContext(ctx).
		Where(repo.ID.Eq(chatID)).
		Select(
			repo.Temperature,
			repo.TopP,
			repo.ReasoningEffort,
			repo.MaxOutputTokens,
			repo.MaxHistoryTokens,
		).
		Updates(chat); err != nil {
		return nil, ctxerrors.Wrap(err, "update chat settings")
	}

	return chat, nil
}

// ownedChat loads a chat owned by userID, translating a missing row to
// commerr.ErrNotFound so callers can propagate it directly.
func (s *Service) ownedChat(
	ctx context.Context,
	chatID, userID uuid.UUID,
) (*models.Chat, error) {
	repo := s.query.Chat

	chat, err := repo.WithContext(ctx).
		Where(repo.ID.Eq(chatID), repo.UserID.Eq(userID)).
		First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, commerr.ErrNotFound
		}

		return nil, ctxerrors.Wrap(err, "get chat")
	}

	return chat, nil
}

// setChatModel persists a chat's model switch so a continued conversation is
// remembered on the model it was last used with.
func (s *Service) setChatModel(
	ctx context.Context,
	chatID uuid.UUID,
	model string,
) error {
	repo := s.query.Chat
	if _, err := repo.WithContext(ctx).
		Where(repo.ID.Eq(chatID)).
		Update(repo.ModelID, model); err != nil {
		return ctxerrors.Wrap(err, "update chat model")
	}

	return nil
}

// setChatTitle persists a chat's title derived from message (trimmed + capped
// by chatTitle), used the one time a still-untitled chat gets its first turn.
func (s *Service) setChatTitle(
	ctx context.Context,
	chatID uuid.UUID,
	message string,
) error {
	repo := s.query.Chat
	if _, err := repo.WithContext(ctx).
		Where(repo.ID.Eq(chatID)).
		Update(repo.Title, chatTitle(message)); err != nil {
		return ctxerrors.Wrap(err, "update chat title")
	}

	return nil
}

// clientForModel resolves the llm client for a model id, guarding a nil
// registry.
//

func (s *Service) clientForModel(model string) (*elelem.Client, bool) {
	if s.models == nil {
		return nil, false
	}

	return s.models.ClientFor(model)
}
