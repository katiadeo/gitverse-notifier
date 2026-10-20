package comments

import (
	"fmt"
	"regexp"
	"strings"

	"gitverse-notifier/pkg/events"
)

var anchorIDRe = regexp.MustCompile(`(?i)#(?:issuecomment|comment)-(\d+)`)

func IDFromURL(values ...string) int {
	for _, raw := range values {
		if match := anchorIDRe.FindStringSubmatch(raw); len(match) == 2 {
			var id int
			if _, err := fmt.Sscanf(match[1], "%d", &id); err == nil && id > 0 {
				return id
			}
		}
	}
	return 0
}

func Identity(c events.CommentInfo) int {
	if c.ID > 0 {
		return c.ID
	}
	return IDFromURL(c.URL)
}

func EnsureID(comment *events.CommentInfo) {
	if comment == nil || comment.ID > 0 {
		return
	}
	comment.ID = IDFromURL(comment.URL)
}

func EnsureURL(comment *events.CommentInfo, prURL string) {
	if comment == nil || strings.TrimSpace(comment.URL) != "" || comment.ID <= 0 {
		return
	}
	base := strings.TrimRight(strings.Split(strings.TrimSpace(prURL), "#")[0], "/")
	if base == "" {
		return
	}
	comment.URL = fmt.Sprintf("%s#issuecomment-%d", base, comment.ID)
}

func NormalizeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	return strings.TrimRight(raw, "/")
}
