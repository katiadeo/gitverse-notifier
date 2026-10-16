package enrichers

import (
	"context"
	"testing"

	"gitverse-notifier/pkg/config"
	"gitverse-notifier/pkg/events"

	"github.com/stretchr/testify/require"
)

func TestGitverseLinks_RewritesAPIHostToUIPullURL(t *testing.T) {
	config.GlobalConfig = &config.Config{GitverseBaseURL: "https://git.example"}
	event := &events.Event{
		Repository: "org/app",
		PullRequest: events.PullRequestInfo{
			URL: "https://git-api.example/org/app/pulls/390",
		},
	}

	err := NewGitverseLinks(nil).Enrich(context.Background(), event)
	require.NoError(t, err)
	require.Equal(t, 390, event.PullRequest.Number)
	require.Equal(t, "https://git.example/org/app/pulls/390", event.PullRequest.URL)
}

func TestGitverseLinks_RewritesRelativePullURL(t *testing.T) {
	config.GlobalConfig = &config.Config{GitverseBaseURL: "https://git.example"}
	event := &events.Event{
		PullRequest: events.PullRequestInfo{
			URL: "/org/app/pulls/412",
		},
	}

	err := NewGitverseLinks(nil).Enrich(context.Background(), event)
	require.NoError(t, err)
	require.Equal(t, 412, event.PullRequest.Number)
	require.Equal(t, "org/app", event.Repository)
	require.Equal(t, "https://git.example/org/app/pulls/412", event.PullRequest.URL)
}
