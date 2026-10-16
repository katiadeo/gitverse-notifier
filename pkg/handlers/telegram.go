package handlers

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"gitverse-notifier/pkg/cache"
	"gitverse-notifier/pkg/events"
	"gitverse-notifier/pkg/integrations/telegram"
	"gitverse-notifier/pkg/logger"
	"gitverse-notifier/pkg/settings"
	"gitverse-notifier/pkg/templates"
)

const (
	defaultTelegramTemplate = "telegram/default"
	telegramMaxRunes        = 4096
)

type TelegramNotify struct {
	client    *telegram.Client
	templates *templates.Engine
	threads   *cache.TelegramPRThreads
}

func NewTelegramNotify(client *telegram.Client, engine *templates.Engine, threads *cache.TelegramPRThreads) *TelegramNotify {
	return &TelegramNotify{client: client, templates: engine, threads: threads}
}

func (a *TelegramNotify) Name() string {
	return "telegram.notify"
}

func (a *TelegramNotify) Ready() bool {
	return a.client != nil
}

func (a *TelegramNotify) Run(ctx context.Context, event *events.Event, rule *settings.ActionRule) error {
	name := strings.TrimSpace(rule.Template)
	if name == "" {
		name = defaultTelegramTemplate
	}

	if event.Type == events.PullRequestComment {
		return a.notifyPRComment(ctx, event, rule, name)
	}

	if event.Type == events.PullRequestClosed {
		if event.PullRequest.EffectiveState() == "merged" {
			if err := a.sendPREvent(ctx, event, name); err != nil {
				return err
			}
		}
		a.deletePRThread(ctx, event)
		return nil
	}

	return a.sendPREvent(ctx, event, name)
}

func (a *TelegramNotify) sendPREvent(ctx context.Context, event *events.Event, name string) error {
	card := a.prCard(event)
	rootID := 0
	if card != nil {
		rootID = card.RootID
	}
	replyTo, bindRoot := telegramThreadPlan(event.Type, rootID)
	if event.Type == events.PullRequestClosed {
		replyTo = 0
		bindRoot = false
	}

	body, err := a.render(ctx, name, event, replyTo > 0)
	if err != nil {
		return err
	}

	id, err := a.client.SendMessage(ctx, body, replyTo)
	if err != nil {
		return err
	}
	if id > 0 {
		if card == nil {
			card = a.prCard(event)
		}
		if bindRoot {
			card.RootID = id
			a.threads.Put(card)
		} else if event.Type != events.PullRequestClosed {
			card.ExtraIDs = appendUniqueID(card.ExtraIDs, id)
			a.threads.Put(card)
		}
	}

	logger.Info(ctx, "successfully sent telegram notification",
		"action", a.Name(),
		"event", event.Type,
		"template", name,
		"repository", event.Repository,
		"reply-to", replyTo,
		"message-id", id,
	)
	return nil
}

func (a *TelegramNotify) notifyPRComment(ctx context.Context, event *events.Event, rule *settings.ActionRule, name string) error {
	if !shouldNotifyPRComment(event.Action) {
		logger.Info(ctx, "skipping telegram comment action",
			"action", a.Name(),
			"event", event.Type,
			"comment-action", event.Action,
			"repository", event.Repository,
		)
		return nil
	}

	if rule.SkipEmpty && templates.StripHTML(event.Comment.Body) == "" {
		logger.Info(ctx, "skipping empty telegram comment",
			"action", a.Name(),
			"event", event.Type,
			"repository", event.Repository,
		)
		return nil
	}

	if event.PullRequest.Number <= 0 {
		logger.Warn(ctx, "telegram PR comment has no pull request number",
			"action", a.Name(),
			"repository", event.Repository,
			"title", event.PullRequest.Title,
			"pr-url", event.PullRequest.URL,
		)
	}

	card := a.prCard(event)
	authors, added := appendUniqueActor(card.Authors, event.Comment.Author)
	if !added && card.CommentID > 0 {
		return nil
	}
	card.Authors = authors
	event.Comments = actorsToComments(card.Authors)

	asReply := card.RootID > 0 && card.RootID != card.CommentID
	body, err := a.render(ctx, name, event, asReply)
	if err != nil {
		return err
	}

	if card.CommentID > 0 {
		if err := a.client.EditMessage(ctx, card.CommentID, body); err == nil || telegram.IsNotModified(err) {
			a.threads.Put(card)
			logger.Info(ctx, "updated telegram comment card",
				"action", a.Name(),
				"repository", event.Repository,
				"pr-number", card.Number,
				"message-id", card.CommentID,
				"authors", len(card.Authors),
			)
			return nil
		} else if !telegram.IsMessageGone(err) {
			logger.Warn(ctx, "failed to edit telegram comment card, sending a new message",
				"action", a.Name(), "err", err, "message-id", card.CommentID)
		}
		card.CommentID = 0
	}

	replyTo := card.RootID
	id, err := a.client.SendMessage(ctx, body, replyTo)
	if err != nil {
		return err
	}
	card.CommentID = id
	if card.RootID == 0 {
		card.RootID = id
	}
	a.threads.Put(card)

	logger.Info(ctx, "successfully sent telegram notification",
		"action", a.Name(),
		"event", event.Type,
		"template", name,
		"repository", event.Repository,
		"reply-to", replyTo,
		"message-id", id,
	)
	return nil
}

func (a *TelegramNotify) deletePRThread(ctx context.Context, event *events.Event) {
	card := a.threads.Get(event.Repository, event.PullRequest.Title, event.PullRequest.Number)
	if card == nil {
		return
	}
	ids := append([]int{card.RootID, card.CommentID}, card.ExtraIDs...)
	if err := a.client.DeleteMessages(ctx, ids); err != nil {
		logger.Warn(ctx, "failed to delete telegram PR thread messages",
			"action", a.Name(),
			"err", err,
			"repository", event.Repository,
			"pr-number", event.PullRequest.Number,
		)
	}
	a.threads.Remove(event.Repository, event.PullRequest.Title, event.PullRequest.Number)
}

func (a *TelegramNotify) prCard(event *events.Event) *cache.TelegramPRCard {
	card := a.threads.Get(event.Repository, event.PullRequest.Title, event.PullRequest.Number)
	if card == nil {
		card = &cache.TelegramPRCard{Repository: event.Repository}
	}
	if event.PullRequest.Number > 0 {
		card.Number = event.PullRequest.Number
	}
	if title := strings.TrimSpace(event.PullRequest.Title); title != "" {
		card.Title = title
	}
	if url := strings.TrimSpace(event.PullRequest.URL); url != "" {
		card.URL = url
	}
	if event.PullRequest.Author != (events.Actor{}) {
		card.PRAuthor = event.PullRequest.Author
	}
	if card.PRAuthor != (events.Actor{}) {
		event.PullRequest.Author = card.PRAuthor
	}
	if card.Number > 0 {
		event.PullRequest.Number = card.Number
	}
	if card.Title != "" {
		event.PullRequest.Title = card.Title
	}
	if card.URL != "" {
		event.PullRequest.URL = card.URL
	}
	return card
}

func (a *TelegramNotify) render(ctx context.Context, name string, event *events.Event, threadReply bool) (string, error) {
	data := templates.DataFromEvent(event)
	data.ThreadReply = threadReply
	body, err := a.templates.Render(name, data)
	if err != nil {
		return "", fmt.Errorf("failed to redener template %s: %w", name, err)
	}
	body = clipTelegramText(body)
	logger.Debug(ctx, "rendered telegram message body",
		"action", a.Name(),
		"body", body,
		"pr-number", event.PullRequest.Number,
		"pr-url", event.PullRequest.URL,
		"thread-reply", threadReply,
	)
	return body, nil
}

func telegramThreadPlan(eventType events.EventType, rootID int) (replyTo int, bindRoot bool) {
	if !isPRThreadEvent(eventType) {
		return 0, false
	}
	if eventType == events.PullRequestOpened {
		return 0, true
	}
	if rootID > 0 {
		return rootID, false
	}
	return 0, true
}

func isPRThreadEvent(t events.EventType) bool {
	switch t {
	case events.PullRequestOpened, events.PullRequestClosed, events.PullRequestComment,
		events.PullRequestReviewRequested, events.PullRequestReviewApproved, events.PullRequestReviewRejected:
		return true
	default:
		return false
	}
}

func appendUniqueActor(authors []events.Actor, actor events.Actor) ([]events.Actor, bool) {
	if actor == (events.Actor{}) {
		return authors, false
	}
	key := actorKey(actor)
	if key == "" {
		return authors, false
	}
	for _, existing := range authors {
		if actorKey(existing) == key {
			return authors, false
		}
	}
	return append(append([]events.Actor(nil), authors...), actor), true
}

func actorKey(actor events.Actor) string {
	if tag := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(actor.TgTag), "@")); tag != "" {
		return "tg:" + tag
	}
	if login := strings.ToLower(strings.TrimSpace(actor.Login)); login != "" {
		return "login:" + login
	}
	return "name:" + strings.ToLower(strings.TrimSpace(actor.Name))
}

func appendUniqueID(ids []int, id int) []int {
	if id <= 0 {
		return ids
	}
	for _, existing := range ids {
		if existing == id {
			return ids
		}
	}
	return append(append([]int(nil), ids...), id)
}

func actorsToComments(authors []events.Actor) []events.CommentInfo {
	out := make([]events.CommentInfo, 0, len(authors))
	for _, author := range authors {
		out = append(out, events.CommentInfo{Author: author})
	}
	return out
}

func shouldNotifyPRComment(action string) bool {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "", "created", "create", "edited", "edit":
		return true
	default:
		return false
	}
}

func clipTelegramText(s string) string {
	if utf8.RuneCountInString(s) <= telegramMaxRunes {
		return s
	}
	return string([]rune(s)[:telegramMaxRunes-1]) + "…"
}
