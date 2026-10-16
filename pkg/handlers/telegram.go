package handlers

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

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
}

func NewTelegramNotify(client *telegram.Client, engine *templates.Engine) *TelegramNotify {
	return &TelegramNotify{client: client, templates: engine}
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

	if event.Type == events.PullRequestComment && !shouldNotifyPRComment(event.Action) {
		logger.Info(ctx, "skipping telegram comment action",
			"action", a.Name(),
			"event", event.Type,
			"comment-action", event.Action,
			"repository", event.Repository,
		)
		return nil
	}

	if rule.SkipEmpty && templates.StripHTML(event.Comment.Body) == "" &&
		strings.ToLower(strings.TrimSpace(event.Action)) != "deleted" {
		logger.Info(ctx, "skipping empty telegram comment",
			"action", a.Name(),
			"event", event.Type,
			"repository", event.Repository,
		)
		return nil
	}

	body, err := a.templates.Render(name, templates.DataFromEvent(event))
	if err != nil {
		return fmt.Errorf("failed to redener template %s: %w", name, err)
	}
	body = clipTelegramText(body)

	if event.Type == events.PullRequestComment && event.PullRequest.Number <= 0 {
		logger.Warn(ctx, "telegram PR comment has no pull request number",
			"action", a.Name(),
			"repository", event.Repository,
			"title", event.PullRequest.Title,
			"pr-url", event.PullRequest.URL,
		)
	}

	logger.Debug(ctx, "rendered telegram message body",
		"action", a.Name(),
		"body", body,
		"pr-number", event.PullRequest.Number,
		"pr-url", event.PullRequest.URL,
	)

	if err := a.client.SendMessage(ctx, body); err != nil {
		return err
	}

	logger.Info(ctx, "successfully sent telegram notification",
		"action", a.Name(),
		"event", event.Type,
		"template", name,
		"repository", event.Repository,
	)
	return nil
}

func shouldNotifyPRComment(action string) bool {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "", "created", "create", "edited", "edit", "deleted", "delete":
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
