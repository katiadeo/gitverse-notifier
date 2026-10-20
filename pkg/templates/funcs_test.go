package templates

import (
	"os"
	"path/filepath"
	"strings"
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

func TestTelegramNotifyAuthor(t *testing.T) {
	config.GlobalConfig = &config.Config{TelegramParseMode: "MarkdownV2"}

	require.Equal(t, "@bob", TelegramNotifyPRAuthor(
		[]events.CommentInfo{{Author: events.Actor{TgTag: "alice"}}},
		events.Actor{TgTag: "bob"},
	))
	require.Equal(t, "@bob", TelegramRichNotifyPRAuthor(
		[]events.CommentInfo{{Author: events.Actor{TgTag: "alice"}}},
		events.Actor{TgTag: "bob"},
	))
	require.Equal(t, "@bob · Comments · 2", TelegramRichCommentSummary(
		[]events.CommentInfo{
			{Author: events.Actor{TgTag: "alice"}},
			{Author: events.Actor{TgTag: "carol"}},
		},
		events.Actor{TgTag: "bob"},
	))
	require.Equal(t, "@bob · Comments · 3 · 2 threads", TelegramRichCommentSummary(
		[]events.CommentInfo{
			{ID: 1, Author: events.Actor{TgTag: "alice"}, Body: "root"},
			{ID: 2, Author: events.Actor{TgTag: "carol"}, Body: "reply", ReplyTo: 1},
			{ID: 3, Author: events.Actor{TgTag: "dave"}, Body: "other"},
		},
		events.Actor{TgTag: "bob"},
	))
	require.Equal(t, "Comments · 1", TelegramRichCommentSummary(
		[]events.CommentInfo{{Author: events.Actor{TgTag: "bob"}}},
		events.Actor{TgTag: "bob"},
	))
	require.Equal(t, "Comments · 1", TelegramRichCommentSummary(
		[]events.CommentInfo{{Author: events.Actor{TgTag: "alice"}}},
		events.Actor{Name: "bob"},
	))
}

func TestCommentThreads(t *testing.T) {
	t.Parallel()

	threads := CommentThreads([]events.CommentInfo{
		{ID: 10, Author: events.Actor{TgTag: "alice"}, Body: "root a"},
		{ID: 11, Author: events.Actor{TgTag: "bob"}, Body: "reply a", ReplyTo: 10},
		{ID: 12, Author: events.Actor{TgTag: "carol"}, Body: "root b"},
		{ID: 13, Author: events.Actor{TgTag: "dave"}, Body: "reply a2", ReplyTo: 11},
	})
	require.Len(t, threads, 2)
	require.Equal(t, 12, threads[0].Root.ID)
	require.Empty(t, threads[0].Replies)
	require.Equal(t, 10, threads[1].Root.ID)
	require.Equal(t, []int{11, 13}, []int{threads[1].Replies[0].ID, threads[1].Replies[1].ID})

	require.Equal(t, "<blockquote>hello</blockquote>", TelegramRichQuote("hello"))
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

func TestStripHTML(t *testing.T) {
	raw := "<p>Запрос на слияние из APP-986 в develop</p><p>уточняет bob<br/>вторая строка</p>"
	require.Equal(t, "Запрос на слияние из APP-986 в develop\nуточняет bob\nвторая строка", StripHTML(raw))
	require.Equal(t, `He said "ok"`, StripHTML("<p>He said &quot;ok&quot;</p>"))
	require.Equal(t, "", StripHTML("<p></p>"))
}

func TestTelegramPlain(t *testing.T) {
	config.GlobalConfig = &config.Config{TelegramParseMode: "MarkdownV2"}
	require.Equal(t, "APP\\-986 \\[FE\\]", TelegramPlain("<p>APP-986 [FE]</p>"))

	config.GlobalConfig.JiraURL = "https://jira.example.com"
	require.Equal(t, "[APP\\-986](https://jira.example.com/browse/APP-986) \\[FE\\]", TelegramPlain("<p>APP-986 [FE]</p>"))
	require.Equal(t, `<a href="https://jira.example.com/browse/APP-1183">APP-1183</a> [FE] form`, TelegramRichPlain("<p>APP-1183 [FE] form</p>"))
	require.Equal(t, `see <a href="https://jira.example.com/browse/APP-1183">APP-1183</a> please`, TelegramRichBody("see APP-1183 please"))
	require.Equal(t, "<pre><code>Testing gitverse bot</code></pre>", TelegramRichBody("```Testing gitverse bot```"))
	require.Equal(t, "<pre><code>line 1\nline 2</code></pre>", TelegramRichBody("```\nline 1\nline 2\n```"))
	require.Equal(t, "Тогда <code>z.array(z.string())</code>", TelegramRichBody("Тогда `z.array(z.string())`"))
	require.Equal(t, "<pre><code>Testing gitverse bot</code></pre>", TelegramRichBody("<pre>Testing gitverse bot</pre>"))
	require.Equal(t, "<b>Префикс</b> <code>handle</code>:", TelegramRichBody("**Префикс** `handle`:"))
	require.Equal(t, "- <b>Префикс</b> <code>handle</code><b>:</b>", TelegramRichBody("- **Префикс** `handle`**:**"))
	require.Equal(t, "@alice", TelegramRichCopyableName(events.Actor{Name: "Alice", TgTag: "alice", URL: "https://git.example/users/24"}))
	require.Equal(t, "bob", TelegramRichCopyableName(events.Actor{Name: "bob", URL: "https://git.example/users/50"}))
	require.Equal(t, `<a href="https://git.example/org/app/pulls/218">#218</a>`, TelegramRichPR(events.PullRequestInfo{
		Number: 218,
		URL:    "https://git.example/org/app/pulls/218",
	}))
	require.Equal(t, "#", TelegramRichPR(events.PullRequestInfo{}))
	require.Equal(t, "<blockquote>please check\nsecond line</blockquote>", TelegramRichQuote("please check\nsecond line"))
	require.Equal(t, "<blockquote><pre><code>line 1\nline 2</code></pre></blockquote>", TelegramRichQuote("```\nline 1\nline 2\n```"))
}

func TestTelegramPRStatePresentation(t *testing.T) {
	previous := config.GlobalConfig
	config.GlobalConfig = &config.Config{}
	t.Cleanup(func() { config.GlobalConfig = previous })

	closed := events.PullRequestInfo{State: "closed", Title: "APP-42 closed work"}
	require.Equal(t, "⚫", TelegramPRStateMarker(closed))
	require.Equal(t, "<s>APP-42 closed work</s>", TelegramRichPRTitle(closed))
	require.Equal(t, "🟣", TelegramPRStateMarker(events.PullRequestInfo{Merged: true}))
	require.Equal(t, "🟢", TelegramPRStateMarker(events.PullRequestInfo{State: "open"}))
}

func TestTelegramPR(t *testing.T) {
	config.GlobalConfig = &config.Config{TelegramParseMode: "MarkdownV2"}
	require.Equal(t, "\\#", TelegramPR(events.PullRequestInfo{}))
	require.Equal(t, "[\\#218](https://git.example/pulls/218)", TelegramPR(events.PullRequestInfo{
		URL: "https://git.example/pulls/218",
	}))
	require.Equal(t, "[\\#218](https://git.example/pulls/218)", TelegramPR(events.PullRequestInfo{
		Number: 218,
		URL:    "https://git.example/pulls/218",
	}))
}

func TestRenderTelegramReviewRequested(t *testing.T) {
	config.GlobalConfig = &config.Config{TelegramParseMode: "MarkdownV2"}

	root := template.New("").Funcs(funcMap())
	mr, err := os.ReadFile(filepath.Join("..", "..", "templates", "telegram", "mr.tmpl"))
	require.NoError(t, err)
	_, err = root.New("telegram/mr").Parse(string(mr))
	require.NoError(t, err)
	content, err := os.ReadFile(filepath.Join("..", "..", "templates", "telegram", "review_requested.tmpl"))
	require.NoError(t, err)
	_, err = root.New("telegram/review_requested").Parse(string(content))
	require.NoError(t, err)

	engine := &Engine{engine: root}
	body, err := engine.Render("telegram/review_requested", Data{
		PullRequest: events.PullRequestInfo{
			Number: 281,
			Title:  "APP-281: deploy guide",
			URL:    "https://git.example/org/app/pulls/281",
			Reviewers: []events.Actor{
				{Name: "alice", TgTag: "alice"},
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, `<p>🟢 <a href="https://git.example/org/app/pulls/281">#281</a> · opened</p><p>APP-281: deploy guide</p><p>review requested · @alice</p>`, strings.TrimSpace(body))
}

func TestRenderTelegramMR(t *testing.T) {
	config.GlobalConfig = &config.Config{TelegramParseMode: "MarkdownV2", JiraURL: "https://jira.example.com"}

	content, err := os.ReadFile(filepath.Join("..", "..", "templates", "telegram", "mr.tmpl"))
	require.NoError(t, err)

	tmpl, err := template.New("telegram/mr").Funcs(funcMap()).Parse(string(content))
	require.NoError(t, err)

	engine := &Engine{engine: tmpl}
	opened, err := engine.Render("telegram/mr", Data{
		PullRequest: events.PullRequestInfo{
			Number: 218,
			Title:  `<p>APP-1237 [FE] Доработка</p>`,
			URL:    "https://git.example/org/app/pulls/218",
			Author: events.Actor{Name: "alice", URL: "https://git.example/alice"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, `<p>🟢 <a href="https://git.example/org/app/pulls/218">#218</a> · opened by <a href="https://git.example/alice">alice</a></p><p><a href="https://jira.example.com/browse/APP-1237">APP-1237</a> [FE] Доработка</p>`, opened)

	body, err := engine.Render("telegram/mr", Data{
		PullRequest: events.PullRequestInfo{
			Number: 412,
			Title:  "APP-1183 [FE] Доработать форму",
			URL:    "https://git.example/org/app/pulls/412",
			Author: events.Actor{Name: "bob", Login: "bob", TgTag: "bob"},
		},
		Comments: []events.CommentInfo{
			{Author: events.Actor{Name: "alice", Login: "alice", TgTag: "alice"}, Body: "please check\nsecond line", URL: "https://git.example/org/app/pulls/412#issuecomment-1"},
			{Author: events.Actor{TgTag: "carol"}, Body: "looks good", Edited: true},
		},
		Approvals: []events.CommentInfo{
			{Author: events.Actor{TgTag: "dave"}},
		},
		MergedBy: events.Actor{TgTag: "eve"},
	})
	require.NoError(t, err)
	require.Equal(t, `<p>🟢 <a href="https://git.example/org/app/pulls/412">#412</a> · opened by <a href="https://t.me/bob">bob</a></p><p><a href="https://jira.example.com/browse/APP-1183">APP-1183</a> [FE] Доработать форму</p><details><summary>@bob · Comments · 2</summary>@carol · edited
<blockquote>looks good</blockquote>@alice · <a href="https://git.example/org/app/pulls/412#issuecomment-1">comment</a>
<blockquote>please check
second line</blockquote></details>`, body)

	merged, err := engine.Render("telegram/mr", Data{
		PullRequest: events.PullRequestInfo{
			Number: 412,
			Merged: true,
			URL:    "https://git.example/org/app/pulls/412",
		},
		MergedBy: events.Actor{TgTag: "eve"},
	})
	require.NoError(t, err)
	require.Equal(t, `<p>🟣 <a href="https://git.example/org/app/pulls/412">#412</a> · merged by <a href="https://t.me/eve">eve</a></p>`, merged)
}

func TestRenderTelegramMR_OnlyThreadsAreExpandable(t *testing.T) {
	config.GlobalConfig = &config.Config{TelegramParseMode: "MarkdownV2"}
	content, err := os.ReadFile(filepath.Join("..", "..", "templates", "telegram", "mr.tmpl"))
	require.NoError(t, err)
	tmpl, err := template.New("telegram/mr").Funcs(funcMap()).Parse(string(content))
	require.NoError(t, err)

	body, err := (&Engine{engine: tmpl}).Render("telegram/mr", Data{
		PullRequest: events.PullRequestInfo{Number: 1, URL: "https://git.example/org/app/pulls/1"},
		Comments: []events.CommentInfo{
			{ID: 1, Author: events.Actor{TgTag: "alice"}, Body: "thread root"},
			{ID: 2, ReplyTo: 1, Author: events.Actor{TgTag: "bob"}, Body: "thread reply"},
			{ID: 3, Author: events.Actor{TgTag: "carol"}, Body: "single comment"},
		},
	})
	require.NoError(t, err)
	require.Contains(t, body, `<details><summary>@alice · 1 replies</summary>`)
	require.Contains(t, body, `&nbsp;&nbsp;↳ @bob`)
	require.Contains(t, body, "@carol\n<blockquote>single comment</blockquote>")
	require.NotContains(t, body, `<details><summary>@carol`)
	require.NotContains(t, body, `· · ·`)
}
