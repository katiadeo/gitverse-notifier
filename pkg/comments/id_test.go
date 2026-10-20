package comments

import (
	"testing"

	"gitverse-notifier/pkg/events"

	"github.com/stretchr/testify/require"
)

func TestIDFromURL(t *testing.T) {
	t.Parallel()
	require.Equal(t, 77, IDFromURL("https://git.example/org/app/pulls/1#issuecomment-77"))
	require.Equal(t, 88, IDFromURL("https://git.example/org/app/pulls/1#comment-88"))
	require.Equal(t, 0, IDFromURL("https://git.example/org/app/pulls/1"))
}

func TestEnsureIDAndURL(t *testing.T) {
	t.Parallel()
	c := events.CommentInfo{URL: "https://git.example/org/app/pulls/1#comment-9"}
	EnsureID(&c)
	require.Equal(t, 9, c.ID)

	c = events.CommentInfo{ID: 12}
	EnsureURL(&c, "https://git.example/org/app/pulls/412")
	require.Equal(t, "https://git.example/org/app/pulls/412#issuecomment-12", c.URL)
}
