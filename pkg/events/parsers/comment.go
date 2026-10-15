package parsers

import (
	"encoding/json"
	"fmt"

	"gitverse-notifier/pkg/events"
)

type issueCommentPayload struct {
	IsPull      bool `json:"isPull"`
	IsPullSnake bool `json:"is_pull"`
	Number      int  `json:"number"`
	Issue       struct {
		Number      int                    `json:"number"`
		Index       int                    `json:"index"`
		Title       string                 `json:"title"`
		Body        string                 `json:"body"`
		State       string                 `json:"state"`
		HTMLURL     string                 `json:"html_url"`
		HTMLUrl     string                 `json:"htmlUrl"`
		User        actorPayload           `json:"user"`
		PullRequest pullRequestBodyPayload `json:"pull_request"`
	} `json:"issue"`
	PullRequest      pullRequestBodyPayload `json:"pullRequest"`
	PullRequestSnake pullRequestBodyPayload `json:"pull_request"`
	Comment          struct {
		Body           string       `json:"body"`
		User           actorPayload `json:"user"`
		HTMLURL        string       `json:"html_url"`
		PullRequestURL string       `json:"pull_request_url"`
	} `json:"comment"`
	Sender actorPayload `json:"sender"`
}

func (p issueCommentPayload) pr() pullRequestBodyPayload {
	if p.PullRequest.Number > 0 || p.PullRequest.Title != "" || p.PullRequest.url() != "" {
		return p.PullRequest
	}
	return p.PullRequestSnake
}

func fillIssueComment(event *events.Event, body []byte) error {
	var payload issueCommentPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("failed to parse issue comment json: %w", err)
	}

	event.Comment = events.CommentInfo{
		Body:   payload.Comment.Body,
		Author: payload.Comment.User.toActor(),
	}
	if event.Comment.Author == (events.Actor{}) {
		event.Comment.Author = payload.Sender.toActor()
	}

	pr := payload.pr()
	isPR := payload.IsPull || payload.IsPullSnake || payload.Issue.Title != "" || payload.Issue.Number > 0 || pr.Number > 0
	if isPR {
		event.PullRequest = events.PullRequestInfo{
			Number: firstPositive(pr.Number, payload.Issue.Number, payload.Issue.Index, payload.Number, prNumberFromRaw(body)),
			Title:  firstNonEmpty(pr.Title, payload.Issue.Title),
			Body:   firstNonEmpty(pr.Body, payload.Issue.Body),
			State:  firstNonEmpty(pr.State, payload.Issue.State),
			URL: firstNonEmpty(
				pr.url(),
				payload.Issue.HTMLURL,
				payload.Issue.HTMLUrl,
				payload.Issue.PullRequest.url(),
				payload.Comment.PullRequestURL,
				payload.Comment.HTMLURL,
			),
			Author:  firstActor(payload.Issue.User, pr.User),
			HeadRef: firstNonEmpty(pr.Head.Ref, pr.Head.Label),
			BaseRef: firstNonEmpty(pr.Base.Ref, pr.Base.Label),
			Merged:  pr.Merged || payload.Issue.PullRequest.Merged,
		}
	}

	return nil
}

func prNumberFromRaw(body []byte) int {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return 0
	}
	return firstPositive(
		jsonPathInt(raw, "number"),
		jsonPathInt(raw, "issue", "number"),
		jsonPathInt(raw, "issue", "index"),
		jsonPathInt(raw, "pullRequest", "number"),
		jsonPathInt(raw, "pull_request", "number"),
		jsonPathInt(raw, "issue", "pullRequest", "number"),
		jsonPathInt(raw, "issue", "pull_request", "number"),
	)
}

func jsonPathInt(cur any, path ...string) int {
	for _, key := range path {
		obj, ok := cur.(map[string]any)
		if !ok {
			return 0
		}
		cur = obj[key]
	}
	switch n := cur.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case json.Number:
		v, _ := n.Int64()
		return int(v)
	default:
		return 0
	}
}

func firstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}
