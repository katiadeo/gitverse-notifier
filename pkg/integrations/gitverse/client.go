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
	title = strings.TrimSpace(title)
	if repository == "" || title == "" {
		return nil, nil
	}

	for _, state := range []string{"open", "all"} {
		var prs []PullRequest
		resp, err := c.resty.R().
			SetContext(ctx).
			SetQueryParams(map[string]string{
				"state":    state,
				"per_page": "100",
			}).
			SetResult(&prs).
			Get("/repos/" + repoAPIPath(repository) + "/pulls")
		if err != nil {
			return nil, fmt.Errorf("failed to make gitverse request: %w", err)
		}
		if resp.IsError() {
			return nil, fmt.Errorf("gitverse api returned %d for %s: %s",
				resp.StatusCode(), resp.Request.URL, string(resp.Body()))
		}
		for i := range prs {
			if strings.TrimSpace(prs[i].Title) == title {
				return &prs[i], nil
			}
		}
	}

	return nil, nil
}
