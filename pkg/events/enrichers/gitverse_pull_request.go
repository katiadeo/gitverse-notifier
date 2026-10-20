package enrichers

import (
	"context"
	"strings"

	"gitverse-notifier/pkg/events"
	"gitverse-notifier/pkg/integrations/gitverse"
	"gitverse-notifier/pkg/logger"
	"gitverse-notifier/pkg/requests"
)

type GitversePullRequest struct {
	dispatcher *requests.RequestsDispatcher
}

func NewGitversePullRequest(dispatcher *requests.RequestsDispatcher) *GitversePullRequest {
	return &GitversePullRequest{dispatcher: dispatcher}
}

func (e *GitversePullRequest) Name() string {
	return "enricher.gitverse-pull-request"
}

func (e *GitversePullRequest) Skip(event *events.Event) bool {
	switch event.Type {
	case events.PullRequestOpened, events.PullRequestClosed, events.PullRequestEdited,
		events.PullRequestSynchronized, events.PullRequestReviewRequested, events.PullRequestReviewRequestRemoved, events.PullRequestReviewApproved, events.PullRequestReviewRejected,
		events.PullRequestReviewComment, events.PullRequestComment:
		return false
	default:
		return true
	}
}

func (e *GitversePullRequest) Enrich(ctx context.Context, event *events.Event) error {
	if event.PullRequest.Number > 0 {
		e.dispatcher.Remember(event.Repository, event.PullRequest.Title, event.PullRequest.Number)
	}

	if event.Repository == "" || (event.PullRequest.Title == "" && event.PullRequest.Number <= 0) {
		logger.Info(ctx, "skip gitverse PR lookup: missing repository or title/number",
			"action", e.Name(), "repository", event.Repository, "title", event.PullRequest.Title,
			"pr-number", event.PullRequest.Number)
		return nil
	}

	pr, err := e.dispatcher.HandleGetPullRequestInfo(ctx, requests.GetPullRequestInfo{
		Repo:      event.Repository,
		Title:     event.PullRequest.Title,
		Number:    event.PullRequest.Number,
		EventType: event.Type,
	})
	if err != nil {
		logger.Warn(ctx, "gitverse PR lookup failed",
			"action", e.Name(), "err", err, "repository", event.Repository,
			"title", event.PullRequest.Title, "pr-number", event.PullRequest.Number)
		return nil
	}
	if pr == nil {
		if event.PullRequest.Number <= 0 {
			logger.Warn(ctx, "gitverse PR lookup found nothing",
				"action", e.Name(), "repository", event.Repository,
				"title", event.PullRequest.Title)
		}
		return nil
	}

	applyPullRequest(event, pr)
	logger.Info(ctx, "enriched pull request from gitverse api",
		"action", e.Name(),
		"repository", event.Repository,
		"pr-number", event.PullRequest.Number,
		"reviewers", len(event.PullRequest.Reviewers),
	)
	return nil
}

func applyPullRequest(event *events.Event, pr *gitverse.PullRequest) {
	pr.Normalize()
	if pr.Number > 0 {
		event.PullRequest.Number = pr.Number
	}
	if title := strings.TrimSpace(pr.Title); title != "" {
		event.PullRequest.Title = title
	}
	if body := strings.TrimSpace(pr.Body); body != "" {
		event.PullRequest.Body = body
	}
	if state := strings.TrimSpace(pr.State); state != "" {
		event.PullRequest.State = state
	}
	mergedByPresent := pr.MergedBy != nil &&
		(pr.MergedBy.ID != 0 || strings.TrimSpace(pr.MergedBy.Login) != "" || strings.TrimSpace(pr.MergedBy.Name) != "")
	event.PullRequest.Merged = pr.Merged ||
		pr.MergedAt != nil ||
		strings.TrimSpace(pr.MergeCommitSHA) != "" ||
		mergedByPresent ||
		strings.EqualFold(strings.TrimSpace(pr.State), "merged")
	if mergedByPresent {
		event.MergedBy = events.Actor{
			ID:    pr.MergedBy.ID,
			Login: firstNonEmpty(pr.MergedBy.Login, pr.MergedBy.Name),
			Name:  firstNonEmpty(pr.MergedBy.Name, pr.MergedBy.Login),
			Email: pr.MergedBy.Email,
			URL:   pr.MergedBy.URL,
		}
	}
	if html := strings.TrimSpace(pr.HTMLURL); html != "" {
		event.PullRequest.URL = html
	}

	if len(pr.RequestedReviewers) > 0 {
		event.PullRequest.Reviewers = make([]events.Actor, 0, len(pr.RequestedReviewers))
		for _, reviewer := range pr.RequestedReviewers {
			event.PullRequest.Reviewers = append(event.PullRequest.Reviewers, events.Actor{
				ID:    reviewer.ID,
				Login: firstNonEmpty(reviewer.Login, reviewer.Name),
				Name:  firstNonEmpty(reviewer.Name, reviewer.Login),
				Email: reviewer.Email,
				URL:   reviewer.URL,
			})
		}
		event.PullRequest.ReviewersExact = true
	}

	if pr.User.ID != 0 || strings.TrimSpace(pr.User.Login) != "" {
		event.PullRequest.Author = events.Actor{
			ID:    pr.User.ID,
			Name:  pr.User.Login,
			Email: pr.User.Email,
			URL:   pr.User.URL,
		}
	}

	if ref := firstNonEmpty(pr.Head.Ref, pr.Head.Label); ref != "" {
		event.PullRequest.HeadRef = ref
	}
	if ref := firstNonEmpty(pr.Base.Ref, pr.Base.Label); ref != "" {
		event.PullRequest.BaseRef = ref
	}

	if event.Branch == "" {
		event.Branch = firstNonEmpty(pr.Head.Ref, pr.Head.Label)
		event.Ref = pr.Head.Ref
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
