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

func TestMemoryPullRequestCache_RebuildsStubFromRememberedNumber(t *testing.T) {
	config.GlobalConfig = &config.Config{CacheTTL: 60, GitverseBaseURL: "https://git.example"}
	c := NewMemoryPullRequestCache()
	title := "APP-1183 form"
	c.Set("org/app", title, 412, gitverse.NewStubPullRequest("org/app", title, 412))
	c.cache.Delete("org/app#412")

	got, ok := c.GetByTitle("org/app", title, 0)
	require.True(t, ok)
	require.Equal(t, 412, got.Number)
	require.Equal(t, "https://git.example/org/app/pulls/412", got.HTMLURL)
}

func TestMemoryPullRequestCache_PersistsNumbersAcrossRestart(t *testing.T) {
	root := t.TempDir()
	config.GlobalConfig = &config.Config{
		Root:            root,
		CacheTTL:        60,
		GitverseBaseURL: "https://git.example",
	}
	title := "APP-1183 [FE] form"
	first := NewMemoryPullRequestCache()
	first.Set("org/app", title, 412, gitverse.NewStubPullRequest("org/app", title, 412))

	second := NewMemoryPullRequestCache()
	got, ok := second.GetByTitle("org/app", title, 0)
	require.True(t, ok)
	require.Equal(t, 412, got.Number)
	require.Equal(t, "https://git.example/org/app/pulls/412", got.HTMLURL)
}
