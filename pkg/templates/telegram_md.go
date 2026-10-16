package templates

import (
	"html"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	htmlPreRe    = regexp.MustCompile(`(?is)<pre[^>]*>(.*?)</pre>`)
	htmlCodeRe   = regexp.MustCompile(`(?is)<code[^>]*>(.*?)</code>`)
	htmlARe      = regexp.MustCompile(`(?is)<a[^>]*href=["']([^"']+)["'][^>]*>(.*?)</a>`)
	htmlStrongRe = regexp.MustCompile(`(?is)<(strong|b)[^>]*>(.*?)</(?:strong|b)>`)
	htmlEmRe     = regexp.MustCompile(`(?is)<(em|i)[^>]*>(.*?)</(?:em|i)>`)
)

// TelegramMD converts GitVerse markdown/HTML comment text into Telegram MarkdownV2,
// keeping inline and fenced code readable instead of escaping backticks.
func TelegramMD(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	return mdToTelegramV2(htmlToMarkdownish(s))
}

func htmlToMarkdownish(s string) string {
	s = htmlBRRe.ReplaceAllString(s, "\n")
	s = htmlPreRe.ReplaceAllStringFunc(s, func(m string) string {
		parts := htmlPreRe.FindStringSubmatch(m)
		if len(parts) < 2 {
			return m
		}
		inner := strings.TrimSpace(StripHTML(parts[1]))
		return "\n```\n" + inner + "\n```\n"
	})
	s = htmlCodeRe.ReplaceAllStringFunc(s, func(m string) string {
		parts := htmlCodeRe.FindStringSubmatch(m)
		if len(parts) < 2 {
			return m
		}
		inner := strings.TrimSpace(StripHTML(parts[1]))
		inner = strings.ReplaceAll(inner, "`", "'")
		return "`" + inner + "`"
	})
	s = htmlARe.ReplaceAllStringFunc(s, func(m string) string {
		parts := htmlARe.FindStringSubmatch(m)
		if len(parts) < 3 {
			return m
		}
		label := strings.TrimSpace(StripHTML(parts[2]))
		if label == "" {
			label = parts[1]
		}
		return "[" + label + "](" + parts[1] + ")"
	})
	s = htmlStrongRe.ReplaceAllStringFunc(s, func(m string) string {
		parts := htmlStrongRe.FindStringSubmatch(m)
		if len(parts) < 3 {
			return m
		}
		return `**` + StripHTML(parts[2]) + `**`
	})
	s = htmlEmRe.ReplaceAllStringFunc(s, func(m string) string {
		parts := htmlEmRe.FindStringSubmatch(m)
		if len(parts) < 3 {
			return m
		}
		return `*` + StripHTML(parts[2]) + `*`
	})
	s = htmlBlockCloseRe.ReplaceAllString(s, "\n")
	s = htmlTagRe.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	s = spaceBeforeNewlineRe.ReplaceAllString(s, "\n")
	s = extraNewlineRe.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

func mdToTelegramV2(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 16)

	for i := 0; i < len(s); {
		if strings.HasPrefix(s[i:], "```") {
			end := strings.Index(s[i+3:], "```")
			if end >= 0 {
				code := s[i+3 : i+3+end]
				code = strings.Trim(code, "\n")
				b.WriteString("```")
				b.WriteString(escapeTelegramCode(code))
				b.WriteString("```")
				i += 6 + end
				continue
			}
		}
		if s[i] == '`' {
			end := strings.IndexByte(s[i+1:], '`')
			if end >= 0 {
				code := s[i+1 : i+1+end]
				b.WriteByte('`')
				b.WriteString(escapeTelegramCode(code))
				b.WriteByte('`')
				i += 2 + end
				continue
			}
		}
		if s[i] == '[' {
			close := strings.IndexByte(s[i+1:], ']')
			if close >= 0 {
				after := i + 2 + close
				if after < len(s) && s[after] == '(' {
					endURL := strings.IndexByte(s[after+1:], ')')
					if endURL >= 0 {
						label := s[i+1 : i+1+close]
						link := s[after+1 : after+1+endURL]
						b.WriteString(TelegramLink(label, link))
						i = after + 2 + endURL
						continue
					}
				}
			}
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		b.WriteString(escapeMarkdownV2(string(r)))
		i += size
	}
	return b.String()
}

func escapeTelegramCode(s string) string {
	return strings.NewReplacer("\\", "\\\\", "`", "\\`").Replace(s)
}
