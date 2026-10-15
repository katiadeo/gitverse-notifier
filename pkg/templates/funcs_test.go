package templates

import (
	"os"
	"path/filepath"
	"testing"
	"text/template"

	"gitverse-notifier/pkg/config"
	"gitverse-notifier/pkg/events"

	"github.com/stretchr/testify/require"
)

func TestTelegramMention(t *testing.T) {
	config.GlobalConfig = &config.Config{TelegramParseMode: "MarkdownV2"}

	require.Equal(t, "@test", TelegramMention(events.Actor{TgTag: "test"}))
	require.Equal(t, "@test", TelegramMention(events.Actor{TgTag: "@test"}))
	require.Equal(t, "@user\\_name", TelegramMention(events.Actor{TgTag: "user_name"}))
	require.Equal(t, "[alice](https://git.example/u/alice)", TelegramMention(events.Actor{
		Name: "alice",
		URL:  "https://git.example/u/alice",
	}))
	require.Equal(t, "", TelegramMention(events.Actor{}))
}

func TestTelegramName(t *testing.T) {
	config.GlobalConfig = &config.Config{TelegramParseMode: "MarkdownV2"}

	require.Equal(t, "[alice\\_tg](https://t.me/alice_tg)", TelegramName(events.Actor{
		Name:  "alice",
		URL:   "https://git.example/u/alice",
		TgTag: "alice_tg",
	}))
	require.Equal(t, "alice", TelegramName(events.Actor{Name: "alice"}))
}

func TestTelegramMentionOther(t *testing.T) {
	config.GlobalConfig = &config.Config{TelegramParseMode: "MarkdownV2"}

	author := events.Actor{ID: 1, Name: "aviktorov", TgTag: "aviktorov"}
	require.Equal(t, "@aviktorov", TelegramMentionOther(author, events.Actor{Name: "nsmirnova"}))
	require.Equal(t, "", TelegramMentionOther(author, events.Actor{ID: 1, Name: "aviktorov"}))
	require.Equal(t, "", TelegramMentionOther(author, events.Actor{Name: "Aviktorov"}))
}

func TestTelegramMentions(t *testing.T) {
	config.GlobalConfig = &config.Config{TelegramParseMode: "MarkdownV2"}

	actors := []events.Actor{
		{TgTag: "one"},
		{Name: "two", URL: "https://git.example/u/two"},
		{TgTag: "three_tag"},
	}
	require.Equal(t, "@one, [two](https://git.example/u/two), @three\\_tag", TelegramMentions(actors))
	require.Equal(t, "", TelegramMentions(nil))
}

func TestStripHTML(t *testing.T) {
	raw := "<p>Запрос на слияние из STKPVT-986 в develop</p><p>уточняет Артем<br/>вторая строка</p>"
	require.Equal(t, "Запрос на слияние из STKPVT-986 в develop\nуточняет Артем\nвторая строка", StripHTML(raw))
	require.Equal(t, `He said "ok"`, StripHTML("<p>He said &quot;ok&quot;</p>"))
	require.Equal(t, "", StripHTML("<p></p>"))
}

func TestTelegramPlain(t *testing.T) {
	config.GlobalConfig = &config.Config{TelegramParseMode: "MarkdownV2"}
	require.Equal(t, "STKPVT\\-986 \\[FE\\]", TelegramPlain("<p>STKPVT-986 [FE]</p>"))
}

func TestTelegramPR(t *testing.T) {
	config.GlobalConfig = &config.Config{TelegramParseMode: "MarkdownV2"}
	require.Equal(t, "PR", TelegramPR(events.PullRequestInfo{}))
	require.Equal(t, "[PR](https://git.example/pulls/218)", TelegramPR(events.PullRequestInfo{
		URL: "https://git.example/pulls/218",
	}))
	require.Equal(t, "[\\#218](https://git.example/pulls/218)", TelegramPR(events.PullRequestInfo{
		Number: 218,
		URL:    "https://git.example/pulls/218",
	}))
}

func TestTelegramPRBranches(t *testing.T) {
	config.GlobalConfig = &config.Config{TelegramParseMode: "MarkdownV2"}
	require.Equal(t, "", TelegramPRBranches(events.PullRequestInfo{HeadRef: "feature"}))
	require.Equal(
		t,
		"[STKPVT\\-1237](https://git.example/src/branch/STKPVT-1237) → [develop](https://git.example/src/branch/develop)",
		TelegramPRBranches(events.PullRequestInfo{
			HeadRef: "STKPVT-1237",
			HeadURL: "https://git.example/src/branch/STKPVT-1237",
			BaseRef: "develop",
			BaseURL: "https://git.example/src/branch/develop",
		}),
	)
}

func TestRenderTelegramReviewRequested(t *testing.T) {
	config.GlobalConfig = &config.Config{TelegramParseMode: "MarkdownV2"}

	content, err := os.ReadFile(filepath.Join("..", "..", "templates", "telegram", "review_requested.tmpl"))
	require.NoError(t, err)

	tmpl, err := template.New("telegram/review_requested").Funcs(funcMap()).Parse(string(content))
	require.NoError(t, err)

	engine := &Engine{engine: tmpl}
	body, err := engine.Render("telegram/review_requested", Data{
		PullRequest: events.PullRequestInfo{
			Number: 281,
			Title:  "STKPVT-281: Гайд по деплою",
			URL:    "https://git.eaisto.at-consulting.ru/bks-frontend/fcs-frontend/pulls/281",
			Reviewers: []events.Actor{
				{Name: "tmukhortova", TgTag: "katiadeo"},
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, `[\#281](https://git.eaisto.at-consulting.ru/bks-frontend/fcs-frontend/pulls/281) · review requested
STKPVT\-281: Гайд по деплою
@katiadeo
`, body)
}

func TestRenderTelegramPRComment(t *testing.T) {
	config.GlobalConfig = &config.Config{TelegramParseMode: "MarkdownV2"}

	content, err := os.ReadFile(filepath.Join("..", "..", "templates", "telegram", "pr_comment.tmpl"))
	require.NoError(t, err)

	tmpl, err := template.New("telegram/pr_comment").Funcs(funcMap()).Parse(string(content))
	require.NoError(t, err)

	engine := &Engine{engine: tmpl}
	body, err := engine.Render("telegram/pr_comment", Data{
		PullRequest: events.PullRequestInfo{
			Number: 237,
			Title:  "STKPVT-738 Доработан nx генератор и его описание в гайде",
			URL:    "https://git.eaisto.at-consulting.ru/bks-frontend/fcs-frontend/pulls/237",
			Author: events.Actor{Name: "tmukhortova", TgTag: "katiadeo"},
		},
		Comment: events.CommentInfo{
			Body:   "super final check",
			Author: events.Actor{Name: "tmukhortova", TgTag: "katiadeo"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, `[\#237](https://git.eaisto.at-consulting.ru/bks-frontend/fcs-frontend/pulls/237) · comment
STKPVT\-738 Доработан nx генератор и его описание в гайде

[katiadeo](https://t.me/katiadeo): super final check
`, body)
}

func TestRenderTelegramPROpened(t *testing.T) {
	config.GlobalConfig = &config.Config{TelegramParseMode: "MarkdownV2"}

	content, err := os.ReadFile(filepath.Join("..", "..", "templates", "telegram", "pr_opened.tmpl"))
	require.NoError(t, err)

	tmpl, err := template.New("telegram/pr_opened").Funcs(funcMap()).Parse(string(content))
	require.NoError(t, err)

	engine := &Engine{engine: tmpl}
	body, err := engine.Render("telegram/pr_opened", Data{
		PullRequest: events.PullRequestInfo{
			Number:  218,
			Title:   `<p>STKPVT-1237 [FE] Доработка</p>`,
			URL:     "https://gitverse.ru/bks-frontend/fcs-frontend/pulls/218",
			HeadRef: "STKPVT-1237",
			HeadURL: "https://gitverse.ru/bks-frontend/fcs-frontend/src/branch/STKPVT-1237",
			BaseRef: "develop",
			BaseURL: "https://gitverse.ru/bks-frontend/fcs-frontend/src/branch/develop",
			Author:  events.Actor{Name: "aviktorov", URL: "https://gitverse.ru/aviktorov"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, `[\#218](https://gitverse.ru/bks-frontend/fcs-frontend/pulls/218) · opened
STKPVT\-1237 \[FE\] Доработка
[aviktorov](https://gitverse.ru/aviktorov) · [STKPVT\-1237](https://gitverse.ru/bks-frontend/fcs-frontend/src/branch/STKPVT-1237) → [develop](https://gitverse.ru/bks-frontend/fcs-frontend/src/branch/develop)
`, body)
}
