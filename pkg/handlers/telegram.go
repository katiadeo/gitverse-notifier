package handlers

import (
	"context"
	"fmt"
	"strings"
	"sync"
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
	mrCardTemplate          = "telegram/mr"
	telegramMaxRunes        = 4096
	telegramRichMaxRunes    = 32000
)

type TelegramNotify struct {
	client    *telegram.Client
	templates *templates.Engine
	threads   *cache.TelegramPRThreads

	commentMu    sync.Mutex
	commentLocks map[string]*sync.Mutex
}

func NewTelegramNotify(client *telegram.Client, engine *templates.Engine, threads *cache.TelegramPRThreads) *TelegramNotify {
	return &TelegramNotify{
		client:       client,
		templates:    engine,
		threads:      threads,
		commentLocks: map[string]*sync.Mutex{},
	}
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

	if isPRCardEvent(event.Type) {
		if name == defaultTelegramTemplate {
			name = mrCardTemplate
		}
		return a.notifyPRCard(ctx, event, rule, name)
	}

	return a.sendPlain(ctx, event, name)
}

func (a *TelegramNotify) sendPlain(ctx context.Context, event *events.Event, name string) error {
	body, err := a.render(ctx, name, event)
	if err != nil {
		return err
	}

	id, err := a.client.SendMessage(ctx, body, 0)
	if err != nil {
		return err
	}

	logger.Info(ctx, "successfully sent telegram notification",
		"action", a.Name(),
		"event", event.Type,
		"template", name,
		"repository", event.Repository,
		"message-id", id,
	)
	return nil
}

func (a *TelegramNotify) render(ctx context.Context, name string, event *events.Event) (string, error) {
	return a.renderClipped(ctx, name, event, telegramMaxRunes)
}

func (a *TelegramNotify) renderRich(ctx context.Context, name string, event *events.Event) (string, error) {
	return a.renderClipped(ctx, name, event, telegramRichMaxRunes)
}

func (a *TelegramNotify) renderClipped(ctx context.Context, name string, event *events.Event, limit int) (string, error) {
	data := templates.DataFromEvent(event)
	body, err := a.templates.Render(name, data)
	if err != nil {
		return "", fmt.Errorf("failed to render template %s: %w", name, err)
	}
	body = clipTelegramText(body, limit)
	logger.Debug(ctx, "rendered telegram message body",
		"action", a.Name(),
		"body", body,
		"pr-number", event.PullRequest.Number,
		"pr-url", event.PullRequest.URL,
	)
	return body, nil
}

func templatesCommentCard(event *events.Event) string {
	return templates.CommentCardMarkdown(templates.DataFromEvent(event))
}

func clipTelegramText(s string, limit int) string {
	if limit <= 1 {
		limit = telegramMaxRunes
	}
	if utf8.RuneCountInString(s) <= limit {
		return s
	}
	return string([]rune(s)[:limit-1]) + "…"
}
