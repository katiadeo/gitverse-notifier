package enrichers

import (
	"context"
	"testing"

	"gitverse-notifier/pkg/config"
	"gitverse-notifier/pkg/events"

	"github.com/stretchr/testify/require"
)

func TestGitverseLinks_RewritesAPIHostToUIPullURL(t *testing.T) {
	config.GlobalConfig = &config.Config{GitverseBaseURL: "https://git.eaisto.at-consulting.ru"}
	event := &events.Event{
		Repository: "bks-frontend/fcs-frontend",
		PullRequest: events.PullRequestInfo{
			URL: "https://git-api.eaisto.at-consulting.ru/bks-frontend/fcs-frontend/pulls/390",
		},
	}

	err := NewGitverseLinks(nil).Enrich(context.Background(), event)
	require.NoError(t, err)
	require.Equal(t, 390, event.PullRequest.Number)
	require.Equal(t, "https://git.eaisto.at-consulting.ru/bks-frontend/fcs-frontend/pulls/390", event.PullRequest.URL)
}
