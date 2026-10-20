package templates

import (
	"html"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	htmlPreRe         = regexp.MustCompile(`(?is)<pre[^>]*>(.*?)</pre>`)
	htmlCodeRe        = regexp.MustCompile(`(?is)<code[^>]*>(.*?)</code>`)
	htmlARe           = regexp.MustCompile(`(?is)<a[^>]*href=["']([^"']+)["'][^>]*>(.*?)</a>`)
	htmlStrongRe      = regexp.MustCompile(`(?is)<(strong|b)[^>]*>(.*?)</(?:strong|b)>`)
	htmlEmRe          = regexp.MustCompile(`(?is)<(em|i)[^>]*>(.*?)</(?:em|i)>`)
	htmlURe           = regexp.MustCompile(`(?is)<u[^>]*>(.*?)</u>`)
	htmlStrikeRe      = regexp.MustCompile(`(?is)<(s|del|strike)[^>]*>(.*?)</(?:s|del|strike)>`)
	mdIssueKeyStartRe = regexp.MustCompile(`^[A-Z][A-Z0-9]+-\d+`)
)

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
	s = htmlURe.ReplaceAllStringFunc(s, func(m string) string {
		parts := htmlURe.FindStringSubmatch(m)
		if len(parts) < 2 {
			return m
		}
		return `__` + StripHTML(parts[1]) + `__`
	})
	s = htmlStrikeRe.ReplaceAllStringFunc(s, func(m string) string {
		parts := htmlStrikeRe.FindStringSubmatch(m)
		if len(parts) < 3 {
			return m
		}
		return `~~` + StripHTML(parts[2]) + `~~`
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
		if inner, next, ok := mdSpan(s, i, "***"); ok {
			writeTGSpan(&b, "*_", "_*", inner)
			i = next
			continue
		}
		if inner, next, ok := mdSpan(s, i, "**"); ok {
			writeTGSpan(&b, "*", "*", inner)
			i = next
			continue
		}
		if inner, next, ok := mdSpan(s, i, "__"); ok {
			writeTGSpan(&b, "__", "__", inner)
			i = next
			continue
		}
		if inner, next, ok := mdSpan(s, i, "~~"); ok {
			writeTGSpan(&b, "~", "~", inner)
			i = next
			continue
		}
		if inner, next, ok := mdEmph(s, i, '*'); ok {
			writeTGSpan(&b, "_", "_", inner)
			i = next
			continue
		}
		if inner, next, ok := mdEmph(s, i, '_'); ok {
			writeTGSpan(&b, "_", "_", inner)
			i = next
			continue
		}
		if key := mdIssueKeyStartRe.FindString(s[i:]); key != "" && mdIssueKeyBoundary(s, i) {
			if url := IssueURL(key); url != "" {
				b.WriteString(TelegramLink(key, url))
			} else {
				b.WriteString(escapeMarkdownV2(key))
			}
			i += len(key)
			continue
		}
		if strings.HasPrefix(s[i:], "```") {
			end := strings.Index(s[i+3:], "```")
			if end >= 0 {
				writeTelegramFence(&b, s[i+3:i+3+end])
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

func writeTelegramFence(b *strings.Builder, body string) {
	lang, code := splitFenceBody(body)
	b.WriteString("```")
	if lang != "" {
		b.WriteString(lang)
	}
	b.WriteByte('\n')
	b.WriteString(escapeTelegramCode(code))
	if !strings.HasSuffix(code, "\n") {
		b.WriteByte('\n')
	}
	b.WriteString("```")
}

func splitFenceBody(body string) (lang, code string) {
	body = strings.Trim(body, "\n")
	nl := strings.IndexByte(body, '\n')
	if nl < 0 {
		return "", body
	}
	first := strings.TrimSpace(body[:nl])
	if !isFenceLang(first) {
		return "", body
	}
	return first, strings.TrimPrefix(body[nl+1:], "\n")
}

func isFenceLang(s string) bool {
	if s == "" || len(s) > 20 {
		return false
	}
	for _, r := range s {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r), r == '+', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}

func writeTGSpan(b *strings.Builder, open, close, inner string) {
	b.WriteString(open)
	b.WriteString(mdToTelegramV2(inner))
	b.WriteString(close)
}

func mdToTelegramHTML(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 16)

	for i := 0; i < len(s); {
		if inner, next, ok := mdSpan(s, i, "***"); ok {
			writeHTMLSpan(&b, "b", mdToTelegramHTML("*"+inner+"*"))
			i = next
			continue
		}
		if inner, next, ok := mdSpan(s, i, "**"); ok {
			writeHTMLSpan(&b, "b", mdToTelegramHTML(inner))
			i = next
			continue
		}
		if inner, next, ok := mdSpan(s, i, "__"); ok {
			writeHTMLSpan(&b, "u", mdToTelegramHTML(inner))
			i = next
			continue
		}
		if inner, next, ok := mdSpan(s, i, "~~"); ok {
			writeHTMLSpan(&b, "s", mdToTelegramHTML(inner))
			i = next
			continue
		}
		if inner, next, ok := mdEmph(s, i, '*'); ok {
			writeHTMLSpan(&b, "i", mdToTelegramHTML(inner))
			i = next
			continue
		}
		if inner, next, ok := mdEmph(s, i, '_'); ok {
			writeHTMLSpan(&b, "i", mdToTelegramHTML(inner))
			i = next
			continue
		}
		if key := mdIssueKeyStartRe.FindString(s[i:]); key != "" && mdIssueKeyBoundary(s, i) {
			if url := IssueURL(key); url != "" {
				b.WriteString(TelegramRichLink(key, url))
			} else {
				b.WriteString(html.EscapeString(key))
			}
			i += len(key)
			continue
		}
		if strings.HasPrefix(s[i:], "```") {
			end := strings.Index(s[i+3:], "```")
			if end >= 0 {
				writeHTMLFence(&b, s[i+3:i+3+end])
				i += 6 + end
				continue
			}
		}
		if s[i] == '`' {
			end := strings.IndexByte(s[i+1:], '`')
			if end >= 0 {
				code := s[i+1 : i+1+end]
				b.WriteString("<code>")
				b.WriteString(html.EscapeString(code))
				b.WriteString("</code>")
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
						b.WriteString(TelegramRichLink(label, link))
						i = after + 2 + endURL
						continue
					}
				}
			}
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		b.WriteString(html.EscapeString(string(r)))
		i += size
	}
	return b.String()
}

func writeHTMLSpan(b *strings.Builder, tag, inner string) {
	b.WriteByte('<')
	b.WriteString(tag)
	b.WriteByte('>')
	b.WriteString(inner)
	b.WriteString("</")
	b.WriteString(tag)
	b.WriteByte('>')
}

func writeHTMLFence(b *strings.Builder, body string) {
	lang, code := splitFenceBody(body)
	b.WriteString("<pre>")
	if lang != "" {
		b.WriteString("<code class=\"language-")
		b.WriteString(html.EscapeString(lang))
		b.WriteString("\">")
	} else {
		b.WriteString("<code>")
	}
	b.WriteString(html.EscapeString(code))
	b.WriteString("</code></pre>")
}

func mdSpan(s string, i int, delim string) (string, int, bool) {
	if !strings.HasPrefix(s[i:], delim) {
		return "", i, false
	}
	end := strings.Index(s[i+len(delim):], delim)
	if end < 0 {
		return "", i, false
	}
	return s[i+len(delim) : i+len(delim)+end], i + 2*len(delim) + end, true
}

func mdEmph(s string, i int, delim byte) (string, int, bool) {
	if i >= len(s) || s[i] != delim {
		return "", i, false
	}
	if i+1 >= len(s) || s[i+1] == delim || s[i+1] == ' ' {
		return "", i, false
	}
	if i > 0 {
		prev, _ := utf8.DecodeLastRuneInString(s[:i])
		if unicode.IsLetter(prev) || unicode.IsDigit(prev) {
			return "", i, false
		}
	}
	for j := i + 1; j < len(s); j++ {
		if s[j] != delim {
			continue
		}
		inner := s[i+1 : j]
		if inner == "" || strings.HasSuffix(inner, " ") {
			return "", i, false
		}
		if j+1 < len(s) {
			next, _ := utf8.DecodeRuneInString(s[j+1:])
			if unicode.IsLetter(next) || unicode.IsDigit(next) {
				continue
			}
		}
		return inner, j + 1, true
	}
	return "", i, false
}

func mdIssueKeyBoundary(s string, i int) bool {
	if i == 0 {
		return true
	}
	r, _ := utf8.DecodeLastRuneInString(s[:i])
	return r != utf8.RuneError && !unicode.IsLetter(r) && !unicode.IsDigit(r)
}
