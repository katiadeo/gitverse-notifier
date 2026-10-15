package enrichers

import (
	"context"
	"strings"

	"gitverse-notifier/pkg/events"
	"gitverse-notifier/pkg/integrations/gitverse"
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
		events.PullRequestSynchronized, events.PullRequestReviewRequested, events.PullRequestReviewApproved, events.PullRequestReviewRejected,
		events.PullRequestReviewComment, events.PullRequestComment:
		return false
	default:
		return true
	}
}

func (e *GitversePullRequest) Enrich(ctx context.Context, event *events.Event) error {
	if event.Repository == "" || event.PullRequest.Title == "" {
		return nil
	}

	pr, err := e.dispatcher.HandleGetPullRequestInfo(ctx, requests.GetPullRequestInfo{
		Repo:      event.Repository,
		Title:     event.PullRequest.Title,
		Number:    event.PullRequest.Number,
		EventType: event.Type,
	})
	if err != nil || pr == nil {
		return err
	}

	applyPullRequest(event, pr)
	return nil
}

func applyPullRequest(event *events.Event, pr *gitverse.PullRequest) {
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
	event.PullRequest.Merged = pr.Merged
	if html := strings.TrimSpace(pr.HTMLURL); html != "" {
		event.PullRequest.URL = html
	}

	if len(pr.RequestedReviewers) > 0 {
		event.PullRequest.Reviewers = make([]events.Actor, 0, len(pr.RequestedReviewers))
		for _, reviewer := range pr.RequestedReviewers {
			event.PullRequest.Reviewers = append(event.PullRequest.Reviewers, events.Actor{
				ID:    reviewer.ID,
				Name:  reviewer.Login,
				Email: reviewer.Email,
				URL:   reviewer.URL,
			})
		}
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
