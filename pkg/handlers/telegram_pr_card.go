package handlers

import (
	"context"
	"strings"
	"sync"

	"gitverse-notifier/pkg/cache"
	"gitverse-notifier/pkg/comments"
	"gitverse-notifier/pkg/events"
	"gitverse-notifier/pkg/integrations/gitverse"
	"gitverse-notifier/pkg/integrations/telegram"
	"gitverse-notifier/pkg/logger"
	"gitverse-notifier/pkg/settings"
	"gitverse-notifier/pkg/templates"
)

func (a *TelegramNotify) notifyPRCard(ctx context.Context, event *events.Event, rule *settings.ActionRule, name string) error {
	if event.Type == events.PullRequestComment {
		if !shouldHandlePRComment(event.Action) {
			logger.Info(ctx, "skipping telegram comment action",
				"action", a.Name(),
				"event", event.Type,
				"comment-action", event.Action,
				"repository", event.Repository,
			)
			return nil
		}
		if !isDeleteComment(event.Action) && rule.SkipEmpty && templates.StripHTML(event.Comment.Body) == "" {
			logger.Info(ctx, "skipping empty telegram comment",
				"action", a.Name(),
				"event", event.Type,
				"repository", event.Repository,
			)
			return nil
		}
	}

	unlock := a.lockComment(event)
	defer unlock()

	card := a.prCard(event)
	if event.PullRequest.Number <= 0 {
		logger.Warn(ctx, "telegram MR card has no pull request number",
			"action", a.Name(),
			"repository", event.Repository,
			"title", event.PullRequest.Title,
			"pr-url", event.PullRequest.URL,
			"event", event.Type,
		)
	}
	hadCard := card.ResolvedMessageID() > 0
	// A comment is an update to a card created by pull_request.opened.  Creating
	// a card from a comment after the cache was lost would produce a second card
	// and make later edits target that new card instead of the original one.
	if event.Type == events.PullRequestComment && !hadCard {
		logger.Warn(ctx, "telegram MR card not found for comment event; not creating a replacement",
			"action", a.Name(),
			"comment-action", event.Action,
			"repository", event.Repository,
			"pr-number", event.PullRequest.Number,
			"comment-id", event.Comment.ID,
		)
		return nil
	}
	beforeComments := len(card.Comments)
	applyPRCardEvent(card, event)
	applyCardToEvent(event, card)

	if event.Type == events.PullRequestComment && (isEditComment(event.Action) || isDeleteComment(event.Action)) {
		if !hadCard {
			logger.Warn(ctx, "telegram MR card not found for comment update",
				"action", a.Name(),
				"event", event.Type,
				"comment-action", event.Action,
				"repository", event.Repository,
				"pr-number", event.PullRequest.Number,
				"comment-id", event.Comment.ID,
			)
			return nil
		}
		if isDeleteComment(event.Action) && len(card.Comments) == beforeComments {
			logger.Warn(ctx, "telegram MR card comment was not removed",
				"action", a.Name(),
				"repository", event.Repository,
				"pr-number", card.Number,
				"comment-id", event.Comment.ID,
				"comment-action", event.Action,
			)
		}
	}

	body, err := a.renderRich(ctx, name, event)
	if err != nil {
		return err
	}

	id := card.ResolvedMessageID()
	if id > 0 {
		editErr := a.editCard(ctx, card, id, body, event)
		if editErr == nil || telegram.IsNotModified(editErr) {
			a.unifyCardMessage(ctx, card, id)
			a.threads.Put(card)
			logger.Info(ctx, "updated telegram MR card",
				"action", a.Name(),
				"event", event.Type,
				"repository", event.Repository,
				"pr-number", card.Number,
				"message-id", id,
				"comments", len(card.Comments),
				"comment-action", event.Action,
			)
			return nil
		}
		if !telegram.IsMessageGone(editErr) {
			logger.Error(ctx, "failed to edit telegram MR card",
				"action", a.Name(), "err", editErr, "message-id", id)
			return editErr
		}
		logger.Warn(ctx, "telegram MR card is gone, sending a replacement",
			"action", a.Name(), "message-id", id)
		card.SetMessageID(0)
	}

	if event.Type == events.PullRequestComment && (isEditComment(event.Action) || isDeleteComment(event.Action)) {
		logger.Warn(ctx, "skipping new telegram MR card for comment update without message id",
			"action", a.Name(),
			"comment-action", event.Action,
			"repository", event.Repository,
		)
		return nil
	}

	newID, rich, err := a.sendCard(ctx, body, event)
	if err != nil {
		return err
	}
	card.SetMessageID(newID)
	card.Rich = rich
	a.threads.Put(card)

	logger.Info(ctx, "successfully sent telegram notification",
		"action", a.Name(),
		"event", event.Type,
		"template", name,
		"repository", event.Repository,
		"message-id", newID,
	)
	return nil
}

func (a *TelegramNotify) prCard(event *events.Event) *cache.TelegramPRCard {
	if n := event.PullRequest.EffectiveNumber(); n > 0 {
		event.PullRequest.Number = n
	}
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
	if event.PullRequest.Number > 0 && strings.TrimSpace(event.PullRequest.URL) == "" {
		stub := gitverse.NewStubPullRequest(event.Repository, event.PullRequest.Title, event.PullRequest.Number)
		if url := strings.TrimSpace(stub.HTMLURL); url != "" {
			event.PullRequest.URL = url
			card.URL = url
		}
	}
	return card
}

func (a *TelegramNotify) editCard(ctx context.Context, card *cache.TelegramPRCard, messageID int, body string, event *events.Event) error {
	err := a.client.EditRich(ctx, messageID, body)
	if err == nil || telegram.IsNotModified(err) {
		card.Rich = true
		return err
	}
	if telegram.IsMessageGone(err) {
		return err
	}
	logger.Warn(ctx, "telegram rich MR card edit failed, trying markdown",
		"action", a.Name(), "err", err, "message-id", messageID)
	return a.client.EditMessage(ctx, messageID, a.cardFallback(event))
}

func (a *TelegramNotify) sendCard(ctx context.Context, body string, event *events.Event) (int, bool, error) {
	id, err := a.client.SendRich(ctx, body, 0)
	if err == nil {
		return id, true, nil
	}
	logger.Warn(ctx, "telegram rich MR card failed, falling back to markdown",
		"action", a.Name(),
		"err", err,
		"repository", event.Repository,
	)
	id, err = a.client.SendMessage(ctx, a.cardFallback(event), 0)
	return id, false, err
}

func (a *TelegramNotify) cardFallback(event *events.Event) string {
	return clipTelegramText(templatesCommentCard(event), telegramMaxRunes)
}

func (a *TelegramNotify) unifyCardMessage(ctx context.Context, card *cache.TelegramPRCard, id int) {
	if card == nil || id <= 0 {
		return
	}
	stale := card.LegacyMessageIDs(id)
	if len(stale) > 0 {
		if err := a.client.DeleteMessages(ctx, stale); err != nil {
			logger.Warn(ctx, "failed to delete leftover telegram MR messages",
				"action", a.Name(), "err", err, "message-id", id)
		}
	}
	card.SetMessageID(id)
}

func (a *TelegramNotify) lockComment(event *events.Event) func() {
	key := cache.CommentLockKey(event.Repository, event.PullRequest.Title, event.PullRequest.Number)
	if key == "" {
		return func() {}
	}
	a.commentMu.Lock()
	if a.commentLocks == nil {
		a.commentLocks = map[string]*sync.Mutex{}
	}
	mu := a.commentLocks[key]
	if mu == nil {
		mu = &sync.Mutex{}
		a.commentLocks[key] = mu
	}
	a.commentMu.Unlock()
	mu.Lock()
	return mu.Unlock
}

func isPRCardEvent(t events.EventType) bool {
	switch t {
	case events.PullRequestOpened, events.PullRequestEdited, events.PullRequestClosed, events.PullRequestComment,
		events.PullRequestReviewRequested, events.PullRequestReviewRequestRemoved,
		events.PullRequestReviewApproved, events.PullRequestReviewRejected:
		return true
	default:
		return false
	}
}

func applyPRCardEvent(card *cache.TelegramPRCard, event *events.Event) {
	if card == nil || event == nil {
		return
	}
	switch event.Type {
	case events.PullRequestOpened:
		card.Opened = true
		applyReviewers(card, event)
	case events.PullRequestEdited:
		applyReviewers(card, event)
	case events.PullRequestComment:
		prURL := event.PullRequest.URL
		if strings.TrimSpace(prURL) == "" {
			prURL = card.URL
		}
		comments.EnsureID(&event.Comment)
		comments.EnsureURL(&event.Comment, prURL)
		if isDeleteComment(event.Action) {
			card.Comments = removeComment(card.Comments, event.Comment)
		} else {
			card.Comments = appendComment(card.Comments, event.Comment, event.Action)
		}
		applyReviewers(card, event)
	case events.PullRequestReviewApproved:
		review := reviewComment(event)
		card.Approvals = upsertByAuthor(card.Approvals, review)
		card.Rejections = removeByAuthor(card.Rejections, review.Author)
		applyReviewers(card, event)
	case events.PullRequestReviewRejected:
		review := reviewComment(event)
		card.Rejections = upsertByAuthor(card.Rejections, review)
		card.Approvals = removeByAuthor(card.Approvals, review.Author)
		applyReviewers(card, event)
	case events.PullRequestReviewRequested, events.PullRequestReviewRequestRemoved:
		applyReviewers(card, event)
	case events.PullRequestClosed:
		card.Closed = true
		merged := event.PullRequest.Merged ||
			event.PullRequest.EffectiveState() == "merged" ||
			event.MergedBy != (events.Actor{})
		if merged {
			card.Merged = true
			event.PullRequest.Merged = true
			event.PullRequest.State = "merged"
			switch {
			case event.MergedBy != (events.Actor{}):
				card.MergedBy = event.MergedBy
			case event.Sender != (events.Actor{}):
				card.MergedBy = event.Sender
			}
		}
	}
}

func applyCardToEvent(event *events.Event, card *cache.TelegramPRCard) {
	if event == nil || card == nil {
		return
	}
	event.Comments = card.Comments
	event.Approvals = card.Approvals
	event.Rejections = card.Rejections
	event.MergedBy = card.MergedBy
	event.PullRequest.Reviewers = append([]events.Actor(nil), card.Reviewers...)
	if card.Merged {
		event.PullRequest.Merged = true
		event.PullRequest.State = "merged"
	} else if card.Closed {
		event.PullRequest.State = "closed"
	}
}
