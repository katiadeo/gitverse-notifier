package parsers

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"gitverse-notifier/pkg/events"
)

var (
	pullsURLRe     = regexp.MustCompile(`(?i)https?://[^\s"'<>]+/(?:pulls?|pull-requests?|merge[_-]?requests?|issues)/\d+`)
	pullsNumberRe  = regexp.MustCompile(`(?i)/(?:pulls?|pull-requests?|merge[_-]?requests?|issues)/(\d+)`)
	apiRepoURLRe   = regexp.MustCompile(`^(https?://[^/]+)(?:/api/v\d+)?/repos/([^/]+/[^/]+)/(?:issues|pulls)/(\d+)`)
	uiRepoURLRe    = regexp.MustCompile(`^(?:https?://[^/]+)?/([^/]+/[^/]+)/(?:pulls?|issues|merge[_-]?requests?)/\d+`)
	issuePathNumRe = regexp.MustCompile(`/issues/(\d+)$`)
	mentionRe      = regexp.MustCompile(`(?:^|[^A-Za-z0-9_])@([A-Za-z][A-Za-z0-9_-]+)`)
)

type issueCommentPayload struct {
	IsPull      bool    `json:"isPull"`
	IsPullSnake bool    `json:"is_pull"`
	Number      flexInt `json:"number"`
	Issue       struct {
		Number           flexInt                `json:"number"`
		Index            flexInt                `json:"index"`
		IID              flexInt                `json:"iid"`
		Title            string                 `json:"title"`
		Body             string                 `json:"body"`
		State            string                 `json:"state"`
		HTMLURL          string                 `json:"html_url"`
		HTMLUrl          string                 `json:"htmlUrl"`
		WebURL           string                 `json:"web_url"`
		WebUrl           string                 `json:"webUrl"`
		URL              string                 `json:"url"`
		User             actorPayload           `json:"user"`
		PullRequest      pullRequestBodyPayload `json:"pullRequest"`
		PullRequestSnake pullRequestBodyPayload `json:"pull_request"`
	} `json:"issue"`
	PullRequest      pullRequestBodyPayload `json:"pullRequest"`
	PullRequestSnake pullRequestBodyPayload `json:"pull_request"`
	Comment          struct {
		Body           string       `json:"body"`
		User           actorPayload `json:"user"`
		HTMLURL        string       `json:"html_url"`
		HTMLUrl        string       `json:"htmlUrl"`
		WebURL         string       `json:"web_url"`
		WebUrl         string       `json:"webUrl"`
		URL            string       `json:"url"`
		PullRequestURL string       `json:"pull_request_url"`
		PullRequestUrl string       `json:"pullRequestUrl"`
		IssueURL       string       `json:"issue_url"`
		IssueUrl       string       `json:"issueUrl"`
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
	issuePR := payload.Issue.PullRequest
	if issuePR.number() == 0 && issuePR.url() == "" && issuePR.Title == "" {
		issuePR = payload.Issue.PullRequestSnake
	}
	isPR := payload.IsPull || payload.IsPullSnake || payload.Issue.Title != "" || payload.Issue.Number.Int() > 0 || payload.Issue.Index.Int() > 0 || pr.number() > 0 || issuePR.number() > 0 || pullsURLRe.Match(body)
	if isPR {
		url := firstNonEmpty(
			pr.url(),
			issuePR.url(),
			toPullHTMLURL(payload.Issue.HTMLURL),
			toPullHTMLURL(payload.Issue.HTMLUrl),
			toPullHTMLURL(payload.Issue.WebURL),
			toPullHTMLURL(payload.Issue.WebUrl),
			toPullHTMLURL(payload.Issue.URL),
			toPullHTMLURL(payload.Comment.PullRequestURL),
			toPullHTMLURL(payload.Comment.PullRequestUrl),
			toPullHTMLURL(payload.Comment.HTMLURL),
			toPullHTMLURL(payload.Comment.HTMLUrl),
			toPullHTMLURL(payload.Comment.WebURL),
			toPullHTMLURL(payload.Comment.WebUrl),
			toPullHTMLURL(payload.Comment.URL),
			toPullHTMLURL(payload.Comment.IssueURL),
			toPullHTMLURL(payload.Comment.IssueUrl),
			prURLFromRaw(body),
		)
		event.PullRequest = events.PullRequestInfo{
			Number: firstPositive(
				pr.number(),
				issuePR.number(),
				payload.Issue.Number.Int(),
				payload.Issue.Index.Int(),
				payload.Issue.IID.Int(),
				payload.Number.Int(),
				prNumberFromRaw(body),
				prNumberFromURL(
					url,
					payload.Comment.HTMLURL,
					payload.Comment.HTMLUrl,
					payload.Comment.IssueURL,
					payload.Comment.IssueUrl,
					payload.Comment.PullRequestURL,
					payload.Comment.PullRequestUrl,
					payload.Comment.URL,
				),
			),
			Title:   firstNonEmpty(pr.Title, payload.Issue.Title),
			Body:    firstNonEmpty(pr.Body, payload.Issue.Body),
			State:   firstNonEmpty(pr.State, payload.Issue.State),
			URL:     url,
			Author:  firstActor(payload.Issue.User, pr.User),
			HeadRef: firstNonEmpty(pr.Head.Ref, pr.Head.Label),
			BaseRef: firstNonEmpty(pr.Base.Ref, pr.Base.Label),
			Merged:  pr.Merged || issuePR.Merged,
		}
		if event.Repository == "" {
			event.Repository = RepoPathFromURL(
				url,
				payload.Issue.HTMLURL,
				payload.Issue.HTMLUrl,
				payload.Issue.URL,
				payload.Comment.IssueURL,
				payload.Comment.IssueUrl,
				payload.Comment.HTMLURL,
				payload.Comment.HTMLUrl,
				payload.Comment.PullRequestURL,
				payload.Comment.URL,
			)
		}
		applyScannedPR(event, body)
	}

	return nil
}

func applyScannedPR(event *events.Event, body []byte) {
	number, url, repo := scanPayloadForPR(body)
	if event.PullRequest.Number <= 0 {
		event.PullRequest.Number = number
	}
	if event.PullRequest.URL == "" {
		event.PullRequest.URL = url
	}
	if event.Repository == "" {
		event.Repository = repo
	}
}

var prNumberKeys = map[string]struct{}{
	"number": {}, "index": {}, "iid": {},
	"pull_number": {}, "pullnumber": {}, "pr_number": {}, "prnumber": {},
	"issue_number": {}, "issuenumber": {}, "issue_index": {}, "issueindex": {},
	"local_id": {}, "localid": {},
}

func scanPayloadForPR(body []byte) (number int, url, repo string) {
	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		return 0, "", ""
	}
	var bestNumber, bestScore int
	var walk func(v any, path []string)
	walk = func(v any, path []string) {
		switch node := v.(type) {
		case map[string]any:
			for key, child := range node {
				walk(child, append(path, key))
			}
		case []any:
			for _, child := range node {
				walk(child, path)
			}
		case string:
			if url == "" {
				if u := toPullHTMLURL(node); u != "" {
					url = u
				} else if match := pullsURLRe.FindString(node); match != "" {
					url = match
				}
			}
			if repo == "" {
				repo = RepoPathFromURL(node)
			}
			if n := prNumberFromURL(node); n > 0 && bestNumber == 0 {
				bestNumber, bestScore = n, 4
			}
		default:
			if len(path) == 0 {
				return
			}
			key := strings.ToLower(path[len(path)-1])
			if _, ok := prNumberKeys[key]; !ok {
				return
			}
			if pathHasFold(path, "comment") && !pathHasFold(path, "issue") && !pathHasFold(path, "pull") {
				return
			}
			n := coercePositiveInt(node)
			if n <= 0 {
				return
			}
			score := 1
			if pathHasFold(path, "issue") || pathHasFold(path, "pull_request") || pathHasFold(path, "pullrequest") {
				score = 3
			} else if len(path) == 1 && key == "number" {
				score = 2
			}
			if score > bestScore {
				bestNumber, bestScore = n, score
			}
		}
	}
	walk(raw, nil)
	return bestNumber, url, repo
}

func pathHasFold(path []string, part string) bool {
	for _, p := range path {
		if strings.Contains(strings.ToLower(p), part) {
			return true
		}
	}
	return false
}

func coercePositiveInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	case json.Number:
		v, _ := n.Int64()
		return int(v)
	case string:
		var parsed int
		if _, err := fmt.Sscanf(strings.TrimSpace(n), "%d", &parsed); err == nil {
			return parsed
		}
	}
	return 0
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

func prNumberFromRaw(body []byte) int {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return 0
	}
	return firstPositive(
		jsonPathInt(raw, "number"),
		jsonPathInt(raw, "issue", "number"),
		jsonPathInt(raw, "issue", "index"),
		jsonPathInt(raw, "issue", "iid"),
		jsonPathInt(raw, "pullRequest", "number"),
		jsonPathInt(raw, "pull_request", "number"),
		jsonPathInt(raw, "issue", "pullRequest", "number"),
		jsonPathInt(raw, "issue", "pull_request", "number"),
		prNumberFromURL(
			string(body),
			jsonPathString(raw, "issue", "url"),
			jsonPathString(raw, "issue", "html_url"),
			jsonPathString(raw, "issue", "htmlUrl"),
			jsonPathString(raw, "comment", "html_url"),
			jsonPathString(raw, "comment", "htmlUrl"),
			jsonPathString(raw, "comment", "issue_url"),
			jsonPathString(raw, "comment", "issueUrl"),
			jsonPathString(raw, "comment", "url"),
			jsonPathString(raw, "comment", "pull_request_url"),
			jsonPathString(raw, "comment", "pullRequestUrl"),
			jsonPathString(raw, "pull_request", "url"),
			jsonPathString(raw, "pull_request", "html_url"),
			jsonPathString(raw, "pull_request", "htmlUrl"),
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
