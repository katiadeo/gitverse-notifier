package parsers

import (
	"encoding/json"
	"fmt"

	"gitverse-notifier/pkg/events"
)

type prBranchPayload struct {
	Ref   string `json:"ref"`
	Label string `json:"label"`
}

type pullRequestBodyPayload struct {
	Number             int             `json:"number"`
	Title              string          `json:"title"`
	Body               string          `json:"body"`
	State              string          `json:"state"`
	Merged             bool            `json:"merged"`
	HTMLURL            string          `json:"html_url"`
	HTMLUrl            string          `json:"htmlUrl"`
	URL                string          `json:"url"`
	User               actorPayload    `json:"user"`
	Head               prBranchPayload `json:"head"`
	Base               prBranchPayload `json:"base"`
	RequestedReviewers []actorPayload  `json:"requested_reviewers"`
}

func (p pullRequestBodyPayload) url() string {
	return firstNonEmpty(p.HTMLURL, p.HTMLUrl, toPullHTMLURL(p.URL))
}

type pullRequestPayload struct {
	Number             int                    `json:"number"`
	PullRequest        pullRequestBodyPayload `json:"pullRequest"`
	PullRequestSnake   pullRequestBodyPayload `json:"pull_request"`
	RequestedReviewer  actorPayload           `json:"requested_reviewer"`
	RequestedReviewers []actorPayload         `json:"requested_reviewers"`
	Review             struct {
		Type    string `json:"type"`
		Content string `json:"content"`
	} `json:"review"`
	Sender actorPayload `json:"sender"`
}

func (p pullRequestPayload) pr() pullRequestBodyPayload {
	if p.PullRequest.Title != "" || p.PullRequest.Number > 0 || p.PullRequest.url() != "" {
		return p.PullRequest
	}
	return p.PullRequestSnake
}

func fillPullRequest(event *events.Event, body []byte) error {
	var payload pullRequestPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("failed to parse pull request json: %w", err)
	}

	pr := payload.pr()
	url := firstNonEmpty(pr.url(), prURLFromRaw(body))
	event.PullRequest = events.PullRequestInfo{
		Number:    firstPositive(pr.Number, payload.Number, prNumberFromRaw(body), prNumberFromURL(url)),
		Title:     pr.Title,
		Body:      pr.Body,
		State:     pr.State,
		Merged:    pr.Merged,
		URL:       url,
		Author:    pr.User.toActor(),
		HeadRef:   firstNonEmpty(pr.Head.Ref, pr.Head.Label),
		BaseRef:   firstNonEmpty(pr.Base.Ref, pr.Base.Label),
		Reviewers: requestedReviewers(event.Type, payload, pr),
	}

	if event.PullRequest.Author == (events.Actor{}) {
		event.PullRequest.Author = event.Sender
	}

	// Для события PullRequestReviewComment приходят пустые сообщения, учитываем их
	if payload.Review.Content != "" || event.Type == events.PullRequestReviewComment {
		event.Comment = events.CommentInfo{
			Body:   payload.Review.Content,
			Author: event.Sender,
		}
	}

	return nil
}
