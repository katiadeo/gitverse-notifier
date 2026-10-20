package gitverse

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"gitverse-notifier/pkg/config"
)

var pullNumberRe = regexp.MustCompile(`/(?:pulls?|issues)/(\d+)`)

type User struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Login    string `json:"login"`
	Type     string `json:"type"`
	Bio      string `json:"bio"`
	Email    string `json:"email"`
	URL      string `json:"url"`
	Location string `json:"location"`
}

type RepoOwner struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	URL   string `json:"url"`
	Type  string `json:"type"`
}

type Repository struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	FullName      string    `json:"full_name"`
	Owner         RepoOwner `json:"owner"`
	Private       bool      `json:"private"`
	Description   string    `json:"description"`
	DefaultBranch string    `json:"default_branch"`
	URL           string    `json:"url"`
	SSHURL        string    `json:"ssh_url"`
}

type PRBranch struct {
	Label  string     `json:"label"`
	Ref    string     `json:"ref"`
	SHA    string     `json:"sha"`
	RepoID int64      `json:"repo_id"`
	Repo   Repository `json:"repo"`
}

type PullRequest struct {
	ID                      int64      `json:"id"`
	URL                     string     `json:"url"`
	HTMLURL                 string     `json:"html_url"`
	HTMLUrl                 string     `json:"htmlUrl"`
	Number                  int        `json:"number"`
	Index                   int        `json:"index"`
	User                    User       `json:"user"`
	Title                   string     `json:"title"`
	Body                    string     `json:"body"`
	RequestedReviewers      []User     `json:"requested_reviewers"`
	RequestedReviewersCamel []User     `json:"requestedReviewers"`
	State                   string     `json:"state"`
	Locked                  bool       `json:"locked"`
	IsDraft                 bool       `json:"is_draft"`
	Comments                int        `json:"comments"`
	Mergeable               bool       `json:"mergeable"`
	Merged                  bool       `json:"merged"`
	MergedAt                *time.Time `json:"merged_at"`
	MergedAtCamel           *time.Time `json:"mergedAt"`
	MergeCommitSHA          string     `json:"merge_commit_sha"`
	MergeCommitSHACamel     string     `json:"mergeCommitSha"`
	MergedBy                *User      `json:"merged_by"`
	MergedByCamel           *User      `json:"mergedBy"`
	MaintainerCanModify     bool       `json:"maintainer_can_modify"`
	Base                    PRBranch   `json:"base"`
	Head                    PRBranch   `json:"head"`
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
	ClosedAt                *time.Time `json:"closed_at"`
}

type CommitGitActor struct {
	Name  string    `json:"name"`
	Email string    `json:"email"`
	Date  time.Time `json:"date"`
}

type CommitTree struct {
	SHA string `json:"sha"`
	URL string `json:"url"`
}

type CommitDetail struct {
	Author    CommitGitActor `json:"author"`
	Committer CommitGitActor `json:"committer"`
	Message   string         `json:"message"`
	Tree      CommitTree     `json:"tree"`
	URL       string         `json:"url"`
}

type CommitUser struct {
	Login            string `json:"login"`
	ID               int64  `json:"id"`
	AvatarURL        string `json:"avatar_url"`
	URL              string `json:"url"`
	HTMLURL          string `json:"html_url"`
	FollowersURL     string `json:"followers_url"`
	FollowingURL     string `json:"following_url"`
	OrganizationsURL string `json:"organizations_url"`
	ReposURL         string `json:"repos_url"`
	Type             string `json:"type"`
	SiteAdmin        bool   `json:"site_admin"`
}

type CommitParent struct {
	URL     string `json:"url"`
	SHA     string `json:"sha"`
	HTMLURL string `json:"html_url"`
}

type CommitStats struct {
	Total     int `json:"total"`
	Additions int `json:"additions"`
	Deletions int `json:"deletions"`
}

type CommitFile struct {
	SHA       string `json:"sha"`
	Filename  string `json:"filename"`
	Status    string `json:"status"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Changes   int    `json:"changes"`
	BlobURL   string `json:"blob_url"`
	Patch     string `json:"patch"`
}

type CommitInfo struct {
	URL       string         `json:"url"`
	SHA       string         `json:"sha"`
	HTMLURL   string         `json:"html_url"`
	Commit    CommitDetail   `json:"commit"`
	Author    *CommitUser    `json:"author"`
	Committer *CommitUser    `json:"committer"`
	Parents   []CommitParent `json:"parents"`
	Stats     CommitStats    `json:"stats"`
	Files     []CommitFile   `json:"files"`
}

func NewStubPullRequest(repository, title string, number int) *PullRequest {
	pr := &PullRequest{Number: number, Title: title}
	base := ""
	if config.GlobalConfig != nil {
		base = strings.TrimRight(config.GlobalConfig.GitverseBaseURL, "/")
	}
	if base != "" && repository != "" && number > 0 {
		pr.HTMLURL = fmt.Sprintf("%s/%s/pulls/%d", base, strings.Trim(repository, "/"), number)
		pr.URL = pr.HTMLURL
	}
	return pr
}

func (p *PullRequest) Normalize() {
	if p == nil {
		return
	}
	if p.Number <= 0 {
		p.Number = p.Index
	}
	if p.Number <= 0 {
		p.Number = pullNumberFromURL(p.HTMLURL, p.HTMLUrl, p.URL)
	}
	if strings.TrimSpace(p.HTMLURL) == "" {
		p.HTMLURL = firstNonEmpty(p.HTMLUrl, p.URL)
	}
	if len(p.RequestedReviewers) == 0 && len(p.RequestedReviewersCamel) > 0 {
		p.RequestedReviewers = p.RequestedReviewersCamel
	}
	if p.MergedAt == nil && p.MergedAtCamel != nil {
		p.MergedAt = p.MergedAtCamel
	}
	if strings.TrimSpace(p.MergeCommitSHA) == "" {
		p.MergeCommitSHA = p.MergeCommitSHACamel
	}
	if p.MergedBy == nil && p.MergedByCamel != nil {
		p.MergedBy = p.MergedByCamel
	}
	if !p.Merged {
		p.Merged = p.MergedAt != nil ||
			strings.TrimSpace(p.MergeCommitSHA) != "" ||
			p.MergedBy != nil ||
			strings.EqualFold(strings.TrimSpace(p.State), "merged")
	}
}

func pullNumberFromURL(values ...string) int {
	for _, value := range values {
		if match := pullNumberRe.FindStringSubmatch(value); len(match) == 2 {
			var n int
			if _, err := fmt.Sscanf(match[1], "%d", &n); err == nil && n > 0 {
				return n
			}
		}
	}
	return 0
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

var (
	draftPrefixRe = regexp.MustCompile(`(?i)^(draft|wip|черновик):\s*`)
	issueKeyRe    = regexp.MustCompile(`\b[A-Z][A-Z0-9]+-\d+\b`)
)

func NormalizePRTitle(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Join(strings.Fields(s), " ")
	s = draftPrefixRe.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

func titlesMatch(got, want string) bool {
	g, w := NormalizePRTitle(got), NormalizePRTitle(want)
	if g == "" || w == "" {
		return false
	}
	if g == w {
		return true
	}
	return strings.Contains(g, w) || strings.Contains(w, g)
}

func IssueKeyFromTitle(title string) string {
	keys := IssueKeysFromTitle(title)
	if len(keys) == 0 {
		return ""
	}
	return keys[0]
}

func IssueKeysFromTitle(title string) []string {
	matches := issueKeyRe.FindAllString(strings.ToUpper(strings.TrimSpace(title)), -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(matches))
	out := make([]string, 0, len(matches))
	for _, key := range matches {
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out
}

func issueKeyIn(title string) string {
	return IssueKeyFromTitle(title)
}

func decodePullRequests(body []byte) ([]PullRequest, error) {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return nil, nil
	}
	if body[0] == '[' {
		var prs []PullRequest
		if err := json.Unmarshal(body, &prs); err != nil {
			return nil, err
		}
		return prs, nil
	}
	var wrap map[string]json.RawMessage
	if err := json.Unmarshal(body, &wrap); err != nil {
		return nil, err
	}
	for _, key := range []string{"data", "items", "pulls", "values", "result", "list", "content"} {
		raw, ok := wrap[key]
		if !ok {
			continue
		}
		prs, err := decodePullRequests(raw)
		if err == nil && len(prs) > 0 {
			return prs, nil
		}
	}
	for _, raw := range wrap {
		var prs []PullRequest
		if json.Unmarshal(raw, &prs) == nil && len(prs) > 0 && strings.TrimSpace(prs[0].Title) != "" {
			return prs, nil
		}
	}
	return nil, nil
}

func uniquePRs(in []*PullRequest) []*PullRequest {
	seen := map[int]struct{}{}
	out := make([]*PullRequest, 0, len(in))
	for _, pr := range in {
		if pr == nil || pr.Number <= 0 {
			continue
		}
		if _, ok := seen[pr.Number]; ok {
			continue
		}
		seen[pr.Number] = struct{}{}
		out = append(out, pr)
	}
	return out
}
