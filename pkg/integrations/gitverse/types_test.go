package gitverse

import (
	"testing"

	"gitverse-notifier/pkg/config"

	"github.com/stretchr/testify/require"
)

func TestPullRequestNormalizeUsesIndexAndURL(t *testing.T) {
	pr := PullRequest{Index: 390, HTMLUrl: "https://git.example/org/app/pulls/390"}
	pr.Normalize()
	require.Equal(t, 390, pr.Number)
	require.Equal(t, "https://git.example/org/app/pulls/390", pr.HTMLURL)

	fromURL := PullRequest{URL: "https://git-api.example/repos/org/app/issues/12"}
	fromURL.Normalize()
	require.Equal(t, 12, fromURL.Number)
}

func TestNormalizePRTitleStripsDraft(t *testing.T) {
	require.Equal(t, "APP-1197 [FE] form", NormalizePRTitle("Draft:  APP-1197 [FE] form"))
	require.Equal(t, "APP-1197 [FE] form", NormalizePRTitle("черновик: APP-1197 [FE] form"))
	require.True(t, titlesMatch("Draft: APP-1197 [FE] form", "APP-1197 [FE] form"))
}

func TestIssueKeysFromTitle(t *testing.T) {
	require.Equal(t, []string{"APP-1167", "APP-1168"}, IssueKeysFromTitle(
		"APP-1167 , APP-1168 [FE] form",
	))
	require.Equal(t, []string{"APP-1", "APP-2", "APP-3", "APP-4"}, IssueKeysFromTitle(
		"APP-1 APP-2, APP-3 / APP-4 form",
	))
	require.Equal(t, "APP-1168", IssueKeyFromTitle("app-1168 first, APP-1167 second"))
	require.Empty(t, IssueKeysFromTitle("no keys here"))
}

func TestNewStubPullRequestBuildsUIURL(t *testing.T) {
	config.GlobalConfig = &config.Config{GitverseBaseURL: "https://git.example"}
	pr := NewStubPullRequest("org/app", "APP-1197 form", 213)
	require.Equal(t, 213, pr.Number)
	require.Equal(t, "https://git.example/org/app/pulls/213", pr.HTMLURL)
}

func TestDecodePullRequestsWrapped(t *testing.T) {
	prs, err := decodePullRequests([]byte(`{"data":[{"number":9,"title":"APP-1197 form"}]}`))
	require.NoError(t, err)
	require.Len(t, prs, 1)
	require.Equal(t, 9, prs[0].Number)
	require.Equal(t, "APP-1197 form", prs[0].Title)
}
