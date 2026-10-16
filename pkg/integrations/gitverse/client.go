package gitverse

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gitverse-notifier/pkg/config"

	"github.com/go-resty/resty/v2"
)

func repoAPIPath(repository string) string {
	return strings.Trim(repository, "/")
}

const versionHeader = "application/vnd.gitverse.object+json;version=1"

type Client struct {
	resty *resty.Client
}

func NewClient() (*Client, error) {
	cfg := config.GlobalConfig
	if cfg.GitverseAPIURL == "" || cfg.GitverseToken == "" {
		return nil, fmt.Errorf("GITVERSE_API_URL and GITVERSE_TOKEN configs required")
	}

	return &Client{
		resty: newRestyClient(cfg.GitverseAPIURL, cfg.GitverseToken),
	}, nil
}

func newRestyClient(baseURL, token string) *resty.Client {
	return resty.New().
		SetBaseURL(baseURL).
		SetAuthToken(token).
		SetHeader("Accept", versionHeader).
		SetTimeout(30 * time.Second)
}

func (c *Client) GetPullRequest(ctx context.Context, repository string, number int) (*PullRequest, error) {
	if repository == "" || number <= 0 {
		return nil, fmt.Errorf("invalid pull request parameters: %s/pulls/%d", repository, number)
	}

	var pr PullRequest
	resp, err := c.resty.R().
		SetContext(ctx).
		SetResult(&pr).
		Get(fmt.Sprintf("/repos/%s/pulls/%d", repoAPIPath(repository), number))

	if err != nil {
		return nil, fmt.Errorf("failed to make gitverse request: %w", err)
	}

	if resp.IsError() {
		return nil, fmt.Errorf("gitverse api returned %d for %s: %s",
			resp.StatusCode(), resp.Request.URL, string(resp.Body()))
	}

	pr.Normalize()
	return &pr, nil
}

func (c *Client) GetCommit(ctx context.Context, repository, sha string) (*CommitInfo, error) {
	if repository == "" || sha == "" {
		return nil, fmt.Errorf("invalid commit parameters: %s/commits/%s", repository, sha)
	}

	var commit CommitInfo
	resp, err := c.resty.R().
		SetContext(ctx).
		SetResult(&commit).
		Get(fmt.Sprintf("/repos/%s/commits/%s", repoAPIPath(repository), sha))

	if err != nil {
		return nil, fmt.Errorf("failed to make gitverse request: %w", err)
	}

	if resp.IsError() {
		return nil, fmt.Errorf("gitverse api returned %d for %s: %s",
			resp.StatusCode(), resp.Request.URL, string(resp.Body()))
	}

	return &commit, nil
}

type createIssueCommentRequest struct {
	Body string `json:"body"`
}

func (c *Client) CreateIssueComment(ctx context.Context, repository string, issueNumber int, body string) error {
	if repository == "" || issueNumber <= 0 {
		return fmt.Errorf("invalid issue comment parameters: %s/issues/%d", repository, issueNumber)
	}

	resp, err := c.resty.R().
		SetContext(ctx).
		SetBody(createIssueCommentRequest{Body: body}).
		Post(fmt.Sprintf("/repos/%s/issues/%d/comments", repoAPIPath(repository), issueNumber))

	if err != nil {
		return fmt.Errorf("failed to make gitverse request: %w", err)
	}

	if resp.IsError() {
		return fmt.Errorf("gitverse api returned %d for %s: %s",
			resp.StatusCode(), resp.Request.URL, string(resp.Body()))
	}

	return nil
}

func (c *Client) FindPullRequestByTitle(ctx context.Context, repository, title string) (*PullRequest, error) {
	want := NormalizePRTitle(title)
	if repository == "" || want == "" {
		return nil, nil
	}

	var fallback []*PullRequest
	wantKey := issueKeyIn(want)

	for _, state := range []string{"open", "opened", "all"} {
		for page := 1; page <= 10; page++ {
			resp, err := c.resty.R().
				SetContext(ctx).
				SetQueryParams(map[string]string{
					"state":    state,
					"per_page": "100",
					"page":     fmt.Sprintf("%d", page),
				}).
				Get("/repos/" + repoAPIPath(repository) + "/pulls")
			if err != nil {
				return nil, fmt.Errorf("failed to make gitverse request: %w", err)
			}
			if resp.IsError() {
				return nil, fmt.Errorf("gitverse api returned %d for %s: %s",
					resp.StatusCode(), resp.Request.URL, string(resp.Body()))
			}
			prs, err := decodePullRequests(resp.Body())
			if err != nil {
				return nil, fmt.Errorf("failed to decode gitverse pulls: %w", err)
			}
			for i := range prs {
				prs[i].Normalize()
				if titlesMatch(prs[i].Title, want) {
					return &prs[i], nil
				}
				if wantKey != "" && strings.Contains(prs[i].Title, wantKey) {
					fallback = append(fallback, &prs[i])
				}
			}
			if len(prs) < 100 {
				break
			}
		}
	}

	if unique := uniquePRs(fallback); len(unique) == 1 {
		return unique[0], nil
	}
	return nil, nil
}
