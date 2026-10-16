package cache

import (
	"testing"

	"gitverse-notifier/pkg/config"
	"gitverse-notifier/pkg/integrations/gitverse"

	"github.com/stretchr/testify/require"
)

func TestMemoryPullRequestCache_RemembersNumberByTitle(t *testing.T) {
	config.GlobalConfig = &config.Config{CacheTTL: 60, GitverseBaseURL: "https://git.example"}
	c := NewMemoryPullRequestCache()
	title := "APP-1197 [FE] form"
	pr := gitverse.NewStubPullRequest("org/app", title, 213)
	c.Set("org/app", title, 213, pr)

	got, ok := c.GetByTitle("org/app", title, 0)
	require.True(t, ok)
	require.Equal(t, 213, got.Number)
	require.Equal(t, "https://git.example/org/app/pulls/213", got.HTMLURL)
}
