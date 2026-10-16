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
