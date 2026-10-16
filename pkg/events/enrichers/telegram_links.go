package enrichers

import (
	"context"
	"strings"

	"gitverse-notifier/pkg/events"
	"gitverse-notifier/pkg/settings"
)

type TelegramLinks struct {
	manager *settings.SettingsManager
}

func NewTelegramLinks(manager *settings.SettingsManager) *TelegramLinks {
	return &TelegramLinks{manager: manager}
}

func (e *TelegramLinks) Name() string {
	return "enricher.telegram-links"
}

func (e *TelegramLinks) Skip(event *events.Event) bool {
	return event == nil
}

func (e *TelegramLinks) Enrich(_ context.Context, event *events.Event) error {
	settings := e.manager.SettingsByRepository(event.Repository)

	e.addTgTag(&event.Sender, settings)
	e.addTgTag(&event.PullRequest.Author, settings)
	e.addTgTag(&event.Comment.Author, settings)
	for i := range len(event.PullRequest.Reviewers) {
		e.addTgTag(&event.PullRequest.Reviewers[i], settings)
	}
	for i := range len(event.Comment.Mentions) {
		e.addTgTag(&event.Comment.Mentions[i], settings)
	}

	return nil
}

func (e *TelegramLinks) addTgTag(actor *events.Actor, settings *settings.RepositorySettings) {
	if actor == nil || settings == nil {
		return
	}
	for _, key := range []string{actor.Login, actor.Name} {
		if tag, ok := lookupTelegramTag(settings.TelegramTags, key); ok {
			actor.TgTag = tag
			return
		}
	}
}

func lookupTelegramTag(tags map[string]string, key string) (string, bool) {
	key = strings.TrimSpace(key)
	if key == "" || len(tags) == 0 {
		return "", false
	}
	if tag, ok := tags[key]; ok {
		return tag, true
	}
	for login, tag := range tags {
		if strings.EqualFold(login, key) {
			return tag, true
		}
	}
	return "", false
}
