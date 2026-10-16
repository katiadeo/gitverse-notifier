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
	default: // MarkdownV2
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

func TelegramIssueURLs(keys []string) string {
	return joinIssueURLs(keys, TelegramLink)
}

func JiraIssueURLs(keys []string) string {
	return joinIssueURLs(keys, JiraLink)
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

func TelegramName(actor events.Actor) string {
	if tag := strings.TrimPrefix(actor.TgTag, "@"); tag != "" {
		return TelegramLink(tag, "https://t.me/"+tag)
	}
	return TelegramLink(actor.Name, actor.URL)
}

func TelegramMentions(actors []events.Actor) string {
	parts := make([]string, 0, len(actors))
	for _, actor := range actors {
		if m := TelegramMention(actor); m != "" {
			parts = append(parts, m)
		}
	}
	return strings.Join(parts, ", ")
}

func TelegramMentionOther(actor, except events.Actor) string {
	if sameActor(actor, except) {
		return ""
	}
	return TelegramMention(actor)
}

func sameActor(a, b events.Actor) bool {
	if a.ID != 0 && a.ID == b.ID {
		return true
	}
	if a.Login != "" && strings.EqualFold(a.Login, b.Login) {
		return true
	}
	if a.Name != "" && strings.EqualFold(a.Name, b.Name) {
		return true
	}
	tagA := strings.TrimPrefix(strings.ToLower(a.TgTag), "@")
	tagB := strings.TrimPrefix(strings.ToLower(b.TgTag), "@")
	return tagA != "" && tagA == tagB
}

func TelegramCommentRecipients(pr events.PullRequestInfo, comment events.CommentInfo) string {
	targets := make([]events.Actor, 0, 1+len(comment.Mentions))
	if pr.Author != (events.Actor{}) {
		targets = append(targets, pr.Author)
	}
	targets = append(targets, comment.Mentions...)

	uniq := make([]events.Actor, 0, len(targets))
	for _, actor := range targets {
		if sameActor(actor, comment.Author) {
			continue
		}
		duplicate := false
		for _, existing := range uniq {
			if sameActor(existing, actor) {
				duplicate = true
				break
			}
		}
		if !duplicate && (actor.Name != "" || actor.Login != "" || actor.TgTag != "") {
			uniq = append(uniq, actor)
		}
	}
	return TelegramMentions(uniq)
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
		return TelegramLink("PR", pr.URL)
	}
	return TelegramEscape("PR")
}

func TelegramPRBranches(pr events.PullRequestInfo) string {
	if strings.TrimSpace(pr.HeadRef) == "" || strings.TrimSpace(pr.BaseRef) == "" {
		return ""
	}
	return TelegramLink(pr.HeadRef, pr.HeadURL) + " → " + TelegramLink(pr.BaseRef, pr.BaseURL)
}
