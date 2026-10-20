package templates

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	"gitverse-notifier/pkg/config"
	"gitverse-notifier/pkg/events"
)

var (
	htmlBRRe             = regexp.MustCompile(`(?i)<br\s*/?>`)
	htmlBlockCloseRe     = regexp.MustCompile(`(?i)</(p|div|li|h[1-6]|tr|table|blockquote)>`)
	htmlTagRe            = regexp.MustCompile(`(?s)<[^>]*>`)
	spaceBeforeNewlineRe = regexp.MustCompile(`[ \t]+\n`)
	extraNewlineRe       = regexp.MustCompile(`\n{3,}`)
)

func JiraEscape(s string) string {
	replacer := strings.NewReplacer(
		"{", "\\{",
		"}", "\\}",
		"[", "\\[",
		"]", "\\]",
		"|", "\\|",
	)
	return replacer.Replace(s)
}

func TelegramEscape(s string) string {
	switch config.GlobalConfig.TelegramParseMode {
	case "HTML":
		return strings.NewReplacer(
			"&", "&amp;",
			"<", "&lt;",
			">", "&gt;",
		).Replace(s)
	case "Markdown":
		return strings.NewReplacer(
			"_", "\\_",
			"*", "\\*",
			"`", "\\`",
			"[", "\\[",
		).Replace(s)
	default:
		return escapeMarkdownV2(s)
	}
}

func escapeMarkdownV2(s string) string {
	return strings.NewReplacer(
		"\\", "\\\\",
		"_", "\\_",
		"*", "\\*",
		"[", "\\[",
		"]", "\\]",
		"(", "\\(",
		")", "\\)",
		"~", "\\~",
		"`", "\\`",
		">", "\\>",
		"#", "\\#",
		"+", "\\+",
		"-", "\\-",
		"=", "\\=",
		"|", "\\|",
		"{", "\\{",
		"}", "\\}",
		".", "\\.",
		"!", "\\!",
	).Replace(s)
}

func joinIssueURLs(keys []string, linkFn func(text, link string) string) string {
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		if key == "" {
			continue
		}
		parts = append(parts, linkFn(key, IssueURL(key)))
	}
	return strings.Join(parts, ", ")
}

func IssueURL(key string) string {
	base := strings.TrimRight(config.GlobalConfig.JiraURL, "/")
	if key == "" || base == "" {
		return ""
	}
	return base + "/browse/" + key
}

func JiraLink(text, link string) string {
	text = JiraEscape(text)
	if link == "" {
		return text
	}
	return fmt.Sprintf("[%s|%s]", text, link)
}

func MarkdownLink(text, link string) string {
	if link == "" {
		return text
	}
	return fmt.Sprintf("[%s](%s)", text, link)
}

func MarkdownIssueURLs(keys []string) string {
	return joinIssueURLs(keys, MarkdownLink)
}

func TelegramLink(text, link string) string {
	text = TelegramEscape(text)
	if link == "" {
		return text
	}
	link = strings.NewReplacer(`\`, `\\`, `)`, `\)`).Replace(link)
	return fmt.Sprintf("[%s](%s)", text, link)
}

func TelegramMention(actor events.Actor) string {
	if tag := strings.TrimPrefix(actor.TgTag, "@"); tag != "" {
		return "@" + TelegramEscape(tag)
	}

	return TelegramName(actor)
}

func TelegramNotifyPRAuthor(comments []events.CommentInfo, author events.Actor) string {
	if strings.TrimPrefix(author.TgTag, "@") == "" {
		return ""
	}
	for _, comment := range comments {
		if !sameTelegramActor(comment.Author, author) {
			return TelegramMention(author)
		}
	}
	return ""
}

func sameTelegramActor(a, b events.Actor) bool {
	aTag := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(a.TgTag), "@"))
	bTag := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(b.TgTag), "@"))
	if aTag != "" && aTag == bTag {
		return true
	}
	if a.Login != "" && b.Login != "" && strings.EqualFold(a.Login, b.Login) {
		return true
	}
	if a.Name != "" && b.Name != "" && strings.EqualFold(a.Name, b.Name) {
		return true
	}
	return false
}

func TelegramName(actor events.Actor) string {
	if tag := strings.TrimPrefix(actor.TgTag, "@"); tag != "" {
		return TelegramLink(tag, "https://t.me/"+tag)
	}
	text := strings.TrimSpace(actor.Name)
	if text == "" {
		text = strings.TrimSpace(actor.Login)
	}
	return TelegramLink(text, actor.URL)
}

func StripHTML(s string) string {
	if s == "" {
		return ""
	}

	s = htmlBRRe.ReplaceAllString(s, "\n")
	s = htmlBlockCloseRe.ReplaceAllString(s, "\n")
	s = htmlTagRe.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	s = spaceBeforeNewlineRe.ReplaceAllString(s, "\n")
	s = extraNewlineRe.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

func TelegramPlain(s string) string {
	s = StripHTML(s)
	if strings.TrimSpace(s) == "" {
		return ""
	}
	return mdToTelegramV2(s)
}

var telegramPRNumberRe = regexp.MustCompile(`/(?:pulls?|issues)/(\d+)`)

func TelegramPR(pr events.PullRequestInfo) string {
	number := pr.Number
	if number <= 0 {
		if match := telegramPRNumberRe.FindStringSubmatch(pr.URL); len(match) == 2 {
			_, _ = fmt.Sscanf(match[1], "%d", &number)
		}
	}
	if number > 0 {
		return TelegramLink(fmt.Sprintf("#%d", number), pr.URL)
	}
	if pr.URL != "" {
		return TelegramLink("#", pr.URL)
	}
	return TelegramEscape("#")
}

func TelegramRichLink(text, link string) string {
	text = html.EscapeString(text)
	if link == "" {
		return text
	}
	return fmt.Sprintf(`<a href="%s">%s</a>`, html.EscapeString(link), text)
}

func TelegramRichPlain(s string) string {
	s = StripHTML(s)
	if strings.TrimSpace(s) == "" {
		return ""
	}
	return linkIssueKeysHTML(html.EscapeString(s))
}

// TelegramPRStateMarker makes the current review state scannable in a busy chat.
// Green is active, purple is merged, and black is a neutral terminal state.
func TelegramPRStateMarker(pr events.PullRequestInfo) string {
	switch strings.ToLower(strings.TrimSpace(pr.EffectiveState())) {
	case "merged":
		return "🟣"
	case "closed":
		return "⚫"
	default:
		return "🟢"
	}
}

func TelegramRichPRTitle(pr events.PullRequestInfo) string {
	title := TelegramRichPlain(pr.Title)
	if title == "" {
		return ""
	}
	if strings.EqualFold(strings.TrimSpace(pr.EffectiveState()), "closed") {
		return "<s>" + title + "</s>"
	}
	return title
}

func TelegramRichBody(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	s = htmlToMarkdownish(s)
	s = strings.ReplaceAll(s, "\\\r\n", "\n")
	s = strings.ReplaceAll(s, "\\\n", "\n")
	return mdToTelegramHTML(s)
}

func TelegramRichQuote(s string) string {
	body := TelegramRichBody(s)
	if body == "" {
		return ""
	}
	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.Trim(body, "\n")
	if body == "" {
		return ""
	}
	return "<blockquote>" + body + "</blockquote>"
}

func TelegramRichCopyableName(actor events.Actor) string {
	if tag := strings.TrimPrefix(actor.TgTag, "@"); tag != "" {
		return "@" + html.EscapeString(tag)
	}
	text := strings.TrimSpace(actor.Name)
	if text == "" {
		text = strings.TrimSpace(actor.Login)
	}
	return html.EscapeString(text)
}

var richIssueKeyRe = regexp.MustCompile(`\b[A-Z][A-Z0-9]+-\d+\b`)

func linkIssueKeysHTML(s string) string {
	return richIssueKeyRe.ReplaceAllStringFunc(s, func(key string) string {
		url := IssueURL(key)
		if url == "" {
			return key
		}
		return TelegramRichLink(key, url)
	})
}

func linkIssueKeysMarkdown(s string) string {
	return richIssueKeyRe.ReplaceAllStringFunc(s, func(key string) string {
		url := IssueURL(key)
		if url == "" {
			return key
		}
		return MarkdownLink(key, url)
	})
}

func TelegramRichName(actor events.Actor) string {
	if tag := strings.TrimPrefix(actor.TgTag, "@"); tag != "" {
		return TelegramRichLink(tag, "https://t.me/"+tag)
	}
	text := strings.TrimSpace(actor.Name)
	if text == "" {
		text = strings.TrimSpace(actor.Login)
	}
	return TelegramRichLink(text, actor.URL)
}

func TelegramRichPR(pr events.PullRequestInfo) string {
	number := pr.Number
	if number <= 0 {
		if match := telegramPRNumberRe.FindStringSubmatch(pr.URL); len(match) == 2 {
			_, _ = fmt.Sscanf(match[1], "%d", &number)
		}
	}
	if number > 0 {
		return TelegramRichLink(fmt.Sprintf("#%d", number), pr.URL)
	}
	if pr.URL != "" {
		return TelegramRichLink("#", pr.URL)
	}
	return html.EscapeString("#")
}

func TelegramRichMention(actor events.Actor) string {
	if tag := strings.TrimPrefix(actor.TgTag, "@"); tag != "" {
		return "@" + html.EscapeString(tag)
	}
	return TelegramRichName(actor)
}

func TelegramRichNotifyPRAuthor(comments []events.CommentInfo, author events.Actor) string {
	if strings.TrimPrefix(author.TgTag, "@") == "" {
		return ""
	}
	for _, comment := range comments {
		if !sameTelegramActor(comment.Author, author) {
			return TelegramRichMention(author)
		}
	}
	return ""
}

func TelegramRichCommentSummary(comments []events.CommentInfo, author events.Actor) string {
	threads := CommentThreads(comments)
	label := html.EscapeString(fmt.Sprintf("Comments · %d", len(comments)))
	if len(threads) > 1 && len(threads) < len(comments) {
		label = html.EscapeString(fmt.Sprintf("Comments · %d · %d threads", len(comments), len(threads)))
	}
	if ping := TelegramRichNotifyPRAuthor(comments, author); ping != "" {
		return ping + " · " + label
	}
	return label
}

func ReverseComments(list []events.CommentInfo) []events.CommentInfo {
	if len(list) == 0 {
		return list
	}
	out := make([]events.CommentInfo, len(list))
	for i, item := range list {
		out[len(list)-1-i] = item
	}
	return out
}

func CommentCardMarkdown(data Data) string {
	var b strings.Builder
	b.WriteString(TelegramPRStateMarker(data.PullRequest))
	b.WriteByte(' ')
	b.WriteString(TelegramPR(data.PullRequest))
	switch data.PullRequest.EffectiveState() {
	case "merged":
		b.WriteString(" · merged")
		if name := TelegramName(data.MergedBy); name != "" {
			b.WriteString(" by ")
			b.WriteString(name)
		}
	case "closed":
		b.WriteString(" · closed")
	default:
		b.WriteString(" · opened")
		if name := TelegramName(data.PullRequest.Author); name != "" {
			b.WriteString(" by ")
			b.WriteString(name)
		}
	}
	b.WriteByte('\n')
	if title := TelegramPlain(data.PullRequest.Title); title != "" {
		if strings.EqualFold(strings.TrimSpace(data.PullRequest.EffectiveState()), "closed") {
			title = "~" + title + "~"
		}
		b.WriteString(title)
		b.WriteByte('\n')
	}
	if len(data.Comments) > 0 {
		if ping := TelegramNotifyPRAuthor(data.Comments, data.PullRequest.Author); ping != "" {
			b.WriteString(ping)
			b.WriteString(" · ")
		}
		fmt.Fprintf(&b, "Comments · %d\n", len(data.Comments))
		for _, comment := range data.Comments {
			b.WriteString("↳ ")
			b.WriteString(TelegramName(comment.Author))
			b.WriteByte('\n')
			if body := TelegramPlain(comment.Body); body != "" {
				b.WriteString(body)
				b.WriteByte('\n')
			}
		}
	}
	if len(data.Approvals) > 0 {
		fmt.Fprintf(&b, "Approved · %d\n", len(data.Approvals))
		for _, approval := range data.Approvals {
			b.WriteString("↳ ")
			b.WriteString(TelegramName(approval.Author))
			b.WriteByte('\n')
		}
	}
	return strings.TrimRight(b.String(), "\n")
}
