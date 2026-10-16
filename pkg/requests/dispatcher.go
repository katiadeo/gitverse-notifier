package requests

import (
	"gitverse-notifier/pkg/cache"
	"gitverse-notifier/pkg/integrations/gitverse"
)

type RequestsDispatcher struct {
	client *gitverse.Client
	prs    cache.PullRequestCache
}

func NewRequestsDispather(client *gitverse.Client, prs cache.PullRequestCache) *RequestsDispatcher {
	return &RequestsDispatcher{client: client, prs: prs}
}

func (d *RequestsDispatcher) Remember(repo, title string, number int) {
	if d == nil || d.prs == nil || number <= 0 || repo == "" {
		return
	}
	d.prs.Set(repo, title, number, gitverse.NewStubPullRequest(repo, title, number))
}
