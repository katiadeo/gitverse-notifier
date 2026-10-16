package parsers

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"gitverse-notifier/pkg/events"
)

var (
	pullsURLRe     = regexp.MustCompile(`https?://[^\s"'<>]+/(?:pulls?|issues)/\d+`)
	pullsNumberRe  = regexp.MustCompile(`/(?:pulls?|issues)/(\d+)`)
	apiRepoURLRe   = regexp.MustCompile(`^(https?://[^/]+)/api/v1/repos/([^/]+/[^/]+)/(?:issues|pulls)/(\d+)`)
	issuePathNumRe = regexp.MustCompile(`/issues/(\d+)$`)
	mentionRe      = regexp.MustCompile(`(?:^|[^A-Za-z0-9_])@([A-Za-z][A-Za-z0-9_-]+)`)
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
		URL         string                 `json:"url"`
		User        actorPayload           `json:"user"`
		PullRequest pullRequestBodyPayload `json:"pull_request"`
	} `json:"issue"`
	PullRequest      pullRequestBodyPayload `json:"pullRequest"`
	PullRequestSnake pullRequestBodyPayload `json:"pull_request"`
	Comment          struct {
		Body           string       `json:"body"`
		User           actorPayload `json:"user"`
		HTMLURL        string       `json:"html_url"`
		URL            string       `json:"url"`
		PullRequestURL string       `json:"pull_request_url"`
		IssueURL       string       `json:"issue_url"`
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
	event.Comment.Mentions = mentionsFrom(payload.Comment.Body, event.Comment.Author)

	pr := payload.pr()
	isPR := payload.IsPull || payload.IsPullSnake || payload.Issue.Title != "" || payload.Issue.Number > 0 || pr.Number > 0 || pullsURLRe.Match(body)
	if isPR {
		url := firstNonEmpty(
			pr.url(),
			toPullHTMLURL(payload.Issue.HTMLURL),
			toPullHTMLURL(payload.Issue.HTMLUrl),
			toPullHTMLURL(payload.Issue.URL),
			payload.Issue.PullRequest.url(),
			toPullHTMLURL(payload.Comment.PullRequestURL),
			toPullHTMLURL(payload.Comment.HTMLURL),
			toPullHTMLURL(payload.Comment.URL),
			toPullHTMLURL(payload.Comment.IssueURL),
			prURLFromRaw(body),
		)
		event.PullRequest = events.PullRequestInfo{
			Number: firstPositive(
				pr.Number,
				payload.Issue.Number,
				payload.Issue.Index,
				payload.Number,
				prNumberFromRaw(body),
				prNumberFromURL(url, payload.Comment.HTMLURL),
			),
			Title:   firstNonEmpty(pr.Title, payload.Issue.Title),
			Body:    firstNonEmpty(pr.Body, payload.Issue.Body),
			State:   firstNonEmpty(pr.State, payload.Issue.State),
			URL:     url,
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
		prNumberFromURL(
			string(body),
			jsonPathString(raw, "issue", "url"),
			jsonPathString(raw, "issue", "html_url"),
			jsonPathString(raw, "comment", "html_url"),
			jsonPathString(raw, "comment", "pull_request_url"),
			jsonPathString(raw, "pull_request", "url"),
			jsonPathString(raw, "pull_request", "html_url"),
		),
	)
}

func toPullHTMLURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if matches := apiRepoURLRe.FindStringSubmatch(raw); len(matches) == 4 {
		return matches[1] + "/" + matches[2] + "/pulls/" + matches[3]
	}
	raw = strings.Split(raw, "#")[0]
	raw = issuePathNumRe.ReplaceAllString(raw, "/pulls/$1")
	if pullsNumberRe.MatchString(raw) {
		return raw
	}
	return ""
}

func prURLFromRaw(body []byte) string {
	return pullURLFrom(string(body))
}

func pullURLFrom(values ...string) string {
	for _, value := range values {
		if match := pullsURLRe.FindString(value); match != "" {
			return match
		}
	}
	return ""
}

func prNumberFromURL(values ...string) int {
	for _, value := range values {
		if match := pullsNumberRe.FindStringSubmatch(value); len(match) == 2 {
			var n int
			if _, err := fmt.Sscanf(match[1], "%d", &n); err == nil && n > 0 {
				return n
			}
		}
	}
	return 0
}

func mentionsFrom(body string, author events.Actor) []events.Actor {
	matches := mentionRe.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return nil
	}

	out := make([]events.Actor, 0, len(matches))
	seen := map[string]struct{}{}
	for _, match := range matches {
		login := match[1]
		key := strings.ToLower(login)
		if key == "" {
			continue
		}
		if author.Login != "" && strings.EqualFold(author.Login, login) {
			continue
		}
		if author.Name != "" && strings.EqualFold(author.Name, login) {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, events.Actor{Name: login, Login: login})
	}
	return out
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
	case string:
		var parsed int
		if _, err := fmt.Sscanf(strings.TrimSpace(n), "%d", &parsed); err == nil && parsed > 0 {
			return parsed
		}
		return prNumberFromURL(n)
	default:
		return 0
	}
}

func jsonPathString(cur any, path ...string) string {
	for _, key := range path {
		obj, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = obj[key]
	}
	s, _ := cur.(string)
	return s
}

func firstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}
