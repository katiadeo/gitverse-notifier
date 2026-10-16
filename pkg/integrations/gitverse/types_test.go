package gitverse

import (
	"testing"

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
	require.Equal(t, "STKPVT-1197 [FE] form", NormalizePRTitle("Draft:  STKPVT-1197 [FE] form"))
	require.Equal(t, "STKPVT-1197 [FE] form", NormalizePRTitle("черновик: STKPVT-1197 [FE] form"))
	require.True(t, titlesMatch("Draft: STKPVT-1197 [FE] form", "STKPVT-1197 [FE] form"))
}

func TestDecodePullRequestsWrapped(t *testing.T) {
	prs, err := decodePullRequests([]byte(`{"data":[{"number":9,"title":"STKPVT-1197 form"}]}`))
	require.NoError(t, err)
	require.Len(t, prs, 1)
	require.Equal(t, 9, prs[0].Number)
	require.Equal(t, "STKPVT-1197 form", prs[0].Title)
}
