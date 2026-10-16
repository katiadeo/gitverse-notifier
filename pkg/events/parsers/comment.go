package parsers

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"gitverse-notifier/pkg/events"
)

var (
	pullsNumberRe  = regexp.MustCompile(`(?i)/(?:pulls?|issues)/(\d+)`)
	apiRepoURLRe   = regexp.MustCompile(`^(https?://[^/]+)(?:/api/v\d+)?/repos/([^/]+/[^/]+)/(?:issues|pulls)/(\d+)`)
	uiRepoURLRe    = regexp.MustCompile(`^https?://[^/]+/([^/]+/[^/]+)/(?:pulls?|issues)/\d+`)
	issuePathNumRe = regexp.MustCompile(`/issues/(\d+)$`)
	mentionRe      = regexp.MustCompile(`(?:^|[^A-Za-z0-9_])@([A-Za-z][A-Za-z0-9_-]+)`)
)

type issueCommentPayload struct {
	IsPull      bool    `json:"isPull"`
	IsPullSnake bool    `json:"is_pull"`
	Number      flexInt `json:"number"`
	Issue       struct {
		Number  flexInt      `json:"number"`
		Title   string       `json:"title"`
		Body    string       `json:"body"`
		State   string       `json:"state"`
		HTMLURL string       `json:"html_url"`
		URL     string       `json:"url"`
		User    actorPayload `json:"user"`
	} `json:"issue"`
	PullRequest      pullRequestBodyPayload `json:"pullRequest"`
	PullRequestSnake pullRequestBodyPayload `json:"pull_request"`
	Comment          struct {
		Body    string       `json:"body"`
		User    actorPayload `json:"user"`
		HTMLURL string       `json:"html_url"`
		URL     string       `json:"url"`
	} `json:"comment"`
	Sender actorPayload `json:"sender"`
}

func (p issueCommentPayload) pr() pullRequestBodyPayload {
	if p.PullRequest.number() > 0 || p.PullRequest.Title != "" || p.PullRequest.url() != "" {
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
	if !payload.IsPull && !payload.IsPullSnake && payload.Issue.Title == "" && pr.number() == 0 {
		return nil
	}

	url := firstNonEmpty(
		pr.url(),
		toPullHTMLURL(payload.Issue.HTMLURL),
		toPullHTMLURL(payload.Issue.URL),
		toPullHTMLURL(payload.Comment.HTMLURL),
		toPullHTMLURL(payload.Comment.URL),
	)
	event.PullRequest = events.PullRequestInfo{
		Number: firstPositive(
			pr.number(),
			payload.Issue.Number.Int(),
			payload.Number.Int(),
			prNumberFromURL(url),
		),
		Title:   firstNonEmpty(pr.Title, payload.Issue.Title),
		Body:    firstNonEmpty(pr.Body, payload.Issue.Body),
		State:   firstNonEmpty(pr.State, payload.Issue.State),
		URL:     url,
		Author:  firstActor(payload.Issue.User, pr.User),
		HeadRef: firstNonEmpty(pr.Head.Ref, pr.Head.Label),
		BaseRef: firstNonEmpty(pr.Base.Ref, pr.Base.Label),
		Merged:  pr.Merged,
	}
	if event.Repository == "" {
		event.Repository = RepoPathFromURL(url, payload.Issue.HTMLURL, payload.Issue.URL, payload.Comment.HTMLURL, payload.Comment.URL)
	}

	return nil
}

// RepoPathFromURL extracts owner/repo from a GitVerse UI or API pull/issue URL.
func RepoPathFromURL(values ...string) string {
	for _, raw := range values {
		raw = strings.TrimSpace(strings.Split(raw, "#")[0])
		if raw == "" {
			continue
		}
		if matches := apiRepoURLRe.FindStringSubmatch(raw); len(matches) == 4 {
			return matches[2]
		}
		if matches := uiRepoURLRe.FindStringSubmatch(raw); len(matches) == 2 {
			return matches[1]
		}
	}
	return ""
}

// PRNumberFromURL extracts a pull/issue number from GitVerse URLs.
func PRNumberFromURL(values ...string) int {
	return prNumberFromURL(values...)
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

func firstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}
