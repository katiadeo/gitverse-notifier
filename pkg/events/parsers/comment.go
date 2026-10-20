package parsers

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"gitverse-notifier/pkg/comments"
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
		Index   flexInt      `json:"index"`
		Title   string       `json:"title"`
		Body    string       `json:"body"`
		State   string       `json:"state"`
		HTMLURL string       `json:"html_url"`
		HTMLUrl string       `json:"htmlUrl"`
		URL     string       `json:"url"`
		User    actorPayload `json:"user"`
	} `json:"issue"`
	PullRequest      pullRequestBodyPayload `json:"pullRequest"`
	PullRequestSnake pullRequestBodyPayload `json:"pull_request"`
	Comment          struct {
		ID             flexInt      `json:"id"`
		CreatedAt      string       `json:"created_at"`
		CreatedAtCamel string       `json:"createdAt"`
		Body           string       `json:"body"`
		User           actorPayload `json:"user"`
		HTMLURL        string       `json:"html_url"`
		HTMLUrl        string       `json:"htmlUrl"`
		URL            string       `json:"url"`
		ReplyTo        flexInt      `json:"reply_to"`
		ReplyToCamel   flexInt      `json:"replyTo"`
		ThreadID       flexInt      `json:"thread_id"`
		ThreadIDCamel  flexInt      `json:"threadId"`
		InReplyTo      flexInt      `json:"in_reply_to"`
		InReplyToCamel flexInt      `json:"inReplyTo"`
		ParentID       flexInt      `json:"parent_id"`
		ParentIDCamel  flexInt      `json:"parentId"`
	} `json:"comment"`
	Changes struct {
		Body struct {
			From string `json:"from"`
		} `json:"body"`
	} `json:"changes"`
	Sender actorPayload `json:"sender"`
}

func (p issueCommentPayload) pr() pullRequestBodyPayload {
	return mergePRBody(p.PullRequest, p.PullRequestSnake)
}

func fillIssueComment(event *events.Event, body []byte) error {
	var payload issueCommentPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("failed to parse issue comment json: %w", err)
	}

	event.Comment = events.CommentInfo{
		ID:           payload.Comment.ID.Int(),
		CreatedAt:    firstNonEmpty(payload.Comment.CreatedAt, payload.Comment.CreatedAtCamel),
		Body:         payload.Comment.Body,
		PreviousBody: payload.Changes.Body.From,
		Author:       payload.Comment.User.toActor(),
		URL:          firstNonEmpty(payload.Comment.HTMLURL, payload.Comment.HTMLUrl),
		ReplyTo:      firstPositive(payload.Comment.ReplyTo.Int(), payload.Comment.ReplyToCamel.Int(), payload.Comment.InReplyTo.Int(), payload.Comment.InReplyToCamel.Int(), payload.Comment.ParentID.Int(), payload.Comment.ParentIDCamel.Int()),
		ThreadID:     firstPositive(payload.Comment.ThreadID.Int(), payload.Comment.ThreadIDCamel.Int()),
	}
	if event.Comment.ID <= 0 {
		event.Comment.ID = comments.IDFromURL(event.Comment.URL, payload.Comment.URL)
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
		toPullHTMLURL(payload.Issue.HTMLUrl),
		toPullHTMLURL(payload.Issue.URL),
		toPullHTMLURL(payload.Comment.HTMLURL),
		toPullHTMLURL(payload.Comment.URL),
	)
	event.PullRequest = events.PullRequestInfo{
		Number: firstPositive(
			pr.number(),
			payload.Issue.Number.Int(),
			payload.Issue.Index.Int(),
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
	event.Comment.URL = commentUIURL(event.Comment.URL, payload.Comment.URL, event.PullRequest.URL, event.Comment.ID)

	return nil
}

func commentUIURL(htmlURL, apiURL, prURL string, id int) string {
	for _, raw := range []string{htmlURL, apiURL} {
		raw = strings.TrimSpace(raw)
		if raw == "" || strings.Contains(raw, "/api/") || strings.Contains(raw, "/repos/") {
			continue
		}
		if pullsNumberRe.MatchString(raw) {
			return raw
		}
	}
	base := strings.TrimRight(strings.Split(strings.TrimSpace(prURL), "#")[0], "/")
	if base != "" && id > 0 {
		return fmt.Sprintf("%s#issuecomment-%d", base, id)
	}
	return ""
}

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
