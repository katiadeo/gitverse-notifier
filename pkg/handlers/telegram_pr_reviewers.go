package handlers

import (
	"strings"

	"gitverse-notifier/pkg/cache"
	"gitverse-notifier/pkg/events"
)

func applyReviewers(card *cache.TelegramPRCard, event *events.Event) {
	reviewers := event.PullRequest.Reviewers
	if event.PullRequest.ReviewersExact {
		card.Reviewers = append([]events.Actor(nil), reviewers...)
		return
	}
	switch event.Type {
	case events.PullRequestReviewRequestRemoved:
		for _, reviewer := range reviewers {
			card.Reviewers = removeActor(card.Reviewers, reviewer)
		}
	default:
		for _, reviewer := range reviewers {
			card.Reviewers, _ = appendUniqueActor(card.Reviewers, reviewer)
		}
	}
}

func removeActor(authors []events.Actor, actor events.Actor) []events.Actor {
	if actor == (events.Actor{}) || len(authors) == 0 {
		return authors
	}
	out := make([]events.Actor, 0, len(authors))
	for _, existing := range authors {
		if sameActor(existing, actor) {
			continue
		}
		out = append(out, existing)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func appendUniqueActor(authors []events.Actor, actor events.Actor) ([]events.Actor, bool) {
	if actor == (events.Actor{}) {
		return authors, false
	}
	for _, existing := range authors {
		if sameActor(existing, actor) {
			return authors, false
		}
	}
	return append(append([]events.Actor(nil), authors...), actor), true
}

func sameActor(a, b events.Actor) bool {
	if a.ID != 0 && b.ID != 0 && a.ID == b.ID {
		return true
	}
	if tag := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(a.TgTag), "@")); tag != "" && tag == strings.ToLower(strings.TrimPrefix(strings.TrimSpace(b.TgTag), "@")) {
		return true
	}
	aLogin := strings.ToLower(strings.TrimSpace(a.Login))
	bLogin := strings.ToLower(strings.TrimSpace(b.Login))
	aName := strings.ToLower(strings.TrimSpace(a.Name))
	bName := strings.ToLower(strings.TrimSpace(b.Name))
	if aLogin != "" && (aLogin == bLogin || aLogin == bName) {
		return true
	}
	if aName != "" && (aName == bName || aName == bLogin) {
		return true
	}
	return false
}
