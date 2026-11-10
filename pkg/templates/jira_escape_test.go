package templates

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJiraEscape(t *testing.T) {
	t.Parallel()

	require.Equal(t, `APP-1339`, JiraEscape("APP-1339"))
	require.Equal(t, `hello \"world\"`, JiraEscape(`hello "world"`))
	require.Equal(t, `\[FE\]`, JiraEscape("[FE]"))
	require.Equal(t, `a\|b`, JiraEscape("a|b"))
	require.Equal(t, `\{x\}`, JiraEscape("{x}"))
}

func TestJiraLinkEscapesQuotesInTitle(t *testing.T) {
	t.Parallel()

	got := JiraLink(
		`APP-1482/APP-1483: Правка формы "Общая информация"`,
		"https://git.example/org/app/pulls/291",
	)
	require.Equal(t,
		`[APP-1482/APP-1483: Правка формы \"Общая информация\"|https://git.example/org/app/pulls/291]`,
		got,
	)
}
