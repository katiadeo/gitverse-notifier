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

	author := events.Actor{ID: 1, Name: "alice", TgTag: "alice"}
	require.Equal(t, "@alice", TelegramMentionOther(author, events.Actor{Name: "bob"}))
	require.Equal(t, "", TelegramMentionOther(author, events.Actor{ID: 1, Name: "alice"}))
	require.Equal(t, "", TelegramMentionOther(author, events.Actor{Name: "Alice"}))
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
				{Name: "alice", TgTag: "alice"},
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, `[\#281](https://git.eaisto.at-consulting.ru/bks-frontend/fcs-frontend/pulls/281) · review requested
STKPVT\-281: Гайд по деплою
@alice
`, body)
}

func TestTelegramMD(t *testing.T) {
	config.GlobalConfig = &config.Config{TelegramParseMode: "MarkdownV2"}

	got := TelegramMD("Тогда `z.array(z.string()).min(1, 'Обязательное поле')`\nИ еще в `controlDirections`")
	require.Equal(t, "Тогда `z.array(z.string()).min(1, 'Обязательное поле')`\nИ еще в `controlDirections`", got)

	got = TelegramMD(`<p>Смотри <code>foo.bar()</code> и <a href="https://example.com">ссылка</a></p>`)
	require.Equal(t, "Смотри `foo.bar()` и [ссылка](https://example.com)", got)

	config.GlobalConfig.JiraURL = "https://jira.example.com"
	got = TelegramMD("#### **STKPVT-1184**")
	require.Equal(t, "*[STKPVT\\-1184](https://jira.example.com/browse/STKPVT-1184)*", got)

	got = TelegramMD("hello **bold**")
	require.Equal(t, "hello *bold*", got)

	got = TelegramMD("hello *italic* and _also_")
	require.Equal(t, "hello _italic_ and _also_", got)

	got = TelegramMD("~~old~~ and <u>under</u>")
	require.Equal(t, "~old~ and __under__", got)

	got = TelegramMD("```\nline 1\nline 2\n```")
	require.Equal(t, "```line 1\nline 2```", got)

	got = TelegramMD("leave foo_bar alone")
	require.Equal(t, "leave foo\\_bar alone", got)
}

func TestTelegramCommentRecipients(t *testing.T) {
	config.GlobalConfig = &config.Config{TelegramParseMode: "MarkdownV2"}

	got := TelegramCommentRecipients(
		events.PullRequestInfo{Author: events.Actor{Login: "bob", Name: "bob", URL: "https://git.example/bob"}},
		events.CommentInfo{
			Author:   events.Actor{Login: "alice", TgTag: "alice"},
			Mentions: []events.Actor{{Login: "bob", Name: "bob", URL: "https://git.example/bob"}},
		},
	)
	require.Equal(t, "[bob](https://git.example/bob)", got)
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
			Number: 412,
			Title:  "STKPVT-1183 [FE] Доработать форму",
			URL:    "https://git.eaisto.at-consulting.ru/bks-frontend/fcs-frontend/pulls/412",
			Author: events.Actor{Name: "bob", Login: "bob", URL: "https://git.example/bob"},
		},
		Comment: events.CommentInfo{
			Body:   "Тогда `z.array(z.string()).min(1, 'Обязательное поле')`",
			Author: events.Actor{Name: "alice", Login: "alice", TgTag: "alice"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, `[\#412](https://git.eaisto.at-consulting.ru/bks-frontend/fcs-frontend/pulls/412)
STKPVT\-1183 \[FE\] Доработать форму

PR author: [bob](https://git.example/bob)
Comment: @alice

Тогда `+"`z.array(z.string()).min(1, 'Обязательное поле')`"+`
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
			Author:  events.Actor{Name: "alice", URL: "https://gitverse.ru/alice"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, `[\#218](https://gitverse.ru/bks-frontend/fcs-frontend/pulls/218) · opened
STKPVT\-1237 \[FE\] Доработка
[alice](https://gitverse.ru/alice) · [STKPVT\-1237](https://gitverse.ru/bks-frontend/fcs-frontend/src/branch/STKPVT-1237) → [develop](https://gitverse.ru/bks-frontend/fcs-frontend/src/branch/develop)
`, body)
}
