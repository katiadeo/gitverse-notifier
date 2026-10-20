package handlers

import (
	"strings"

	"gitverse-notifier/pkg/comments"
	"gitverse-notifier/pkg/events"
	"gitverse-notifier/pkg/templates"
)

func reviewComment(event *events.Event) events.CommentInfo {
	author := event.Sender
	if author == (events.Actor{}) {
		author = event.Comment.Author
	}
	comment := events.CommentInfo{
		ID:     event.Comment.ID,
		Author: author,
		Body:   event.Comment.Body,
		URL:    event.Comment.URL,
	}
	comments.EnsureURL(&comment, event.PullRequest.URL)
	return comment
}

func upsertByAuthor(list []events.CommentInfo, comment events.CommentInfo) []events.CommentInfo {
	if comment.Author == (events.Actor{}) && strings.TrimSpace(comment.Body) == "" {
		return list
	}
	out := append([]events.CommentInfo(nil), list...)
	for i, existing := range out {
		if sameActor(existing.Author, comment.Author) {
			out[i] = comment
			return out
		}
	}
	return append(out, comment)
}

func removeByAuthor(list []events.CommentInfo, author events.Actor) []events.CommentInfo {
	if len(list) == 0 || author == (events.Actor{}) {
		return list
	}
	out := make([]events.CommentInfo, 0, len(list))
	for _, comment := range list {
		if !sameActor(comment.Author, author) {
			out = append(out, comment)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func appendComment(list []events.CommentInfo, comment events.CommentInfo, action string) []events.CommentInfo {
	if comment.Author == (events.Actor{}) && strings.TrimSpace(comment.Body) == "" {
		return list
	}
	comments.EnsureID(&comment)
	edited := isEditComment(action)
	if idx := findCommentIndex(list, comment); idx >= 0 {
		out := append([]events.CommentInfo(nil), list...)
		out[idx] = mergeComment(list[idx], comment, edited || list[idx].Edited)
		return dedupeCommentsByIdentity(out)
	}
	if edited {
		// Never use the author alone as an identity: one author can leave several
		// comments in a PR, and GitVerse can omit the numeric comment ID. In that
		// case the creation timestamp parsed above distinguishes the comments.
		comment.Edited = true
	}
	return dedupeCommentsByIdentity(append(append([]events.CommentInfo(nil), list...), comment))
}

func dedupeCommentsByIdentity(list []events.CommentInfo) []events.CommentInfo {
	if len(list) < 2 {
		return list
	}
	out := make([]events.CommentInfo, 0, len(list))
	seen := map[int]int{}
	for _, comment := range list {
		id := comments.Identity(comment)
		if id <= 0 {
			out = append(out, comment)
			continue
		}
		if idx, ok := seen[id]; ok {
			out[idx] = mergeComment(out[idx], comment, comment.Edited || out[idx].Edited)
			continue
		}
		seen[id] = len(out)
		out = append(out, comment)
	}
	return out
}

func mergeComment(existing, incoming events.CommentInfo, edited bool) events.CommentInfo {
	out := incoming
	out.Edited = edited
	if out.ID <= 0 {
		out.ID = comments.Identity(existing)
	}
	if strings.TrimSpace(out.URL) == "" {
		out.URL = existing.URL
	}
	if out.ReplyTo <= 0 {
		out.ReplyTo = existing.ReplyTo
	}
	if out.ThreadID <= 0 {
		out.ThreadID = existing.ThreadID
	}
	out.Author = mergeActor(existing.Author, incoming.Author)
	return out
}

func mergeActor(existing, incoming events.Actor) events.Actor {
	out := incoming
	if out.ID == 0 {
		out.ID = existing.ID
	}
	if strings.TrimSpace(out.Login) == "" {
		out.Login = existing.Login
	}
	if strings.TrimSpace(out.Name) == "" {
		out.Name = existing.Name
	}
	if strings.TrimSpace(out.TgTag) == "" {
		out.TgTag = existing.TgTag
	}
	if strings.TrimSpace(out.URL) == "" {
		out.URL = existing.URL
	}
	if strings.TrimSpace(out.Email) == "" {
		out.Email = existing.Email
	}
	return out
}

func findCommentIndex(list []events.CommentInfo, comment events.CommentInfo) int {
	id := comments.Identity(comment)
	if id > 0 {
		for i, existing := range list {
			if comments.Identity(existing) == id {
				return i
			}
		}
	}
	url := comments.NormalizeURL(comment.URL)
	if url != "" {
		for i, existing := range list {
			if comments.NormalizeURL(existing.URL) == url {
				return i
			}
		}
	}
	if idx := findCommentIndexByCreatedAt(list, comment); idx >= 0 {
		return idx
	}
	return findCommentIndexByPreviousBody(list, comment)
}

func findCommentIndexByPreviousBody(list []events.CommentInfo, comment events.CommentInfo) int {
	previous := strings.TrimSpace(templates.StripHTML(comment.PreviousBody))
	if previous == "" {
		return -1
	}

	match := -1
	for i, existing := range list {
		if !sameActor(existing.Author, comment.Author) || strings.TrimSpace(templates.StripHTML(existing.Body)) != previous {
			continue
		}
		if match >= 0 {
			// The old body is not a unique identity; avoid changing the wrong
			// comment when identical messages were posted by the same author.
			return -1
		}
		match = i
	}
	return match
}

func findCommentIndexByCreatedAt(list []events.CommentInfo, comment events.CommentInfo) int {
	createdAt := strings.TrimSpace(comment.CreatedAt)
	if createdAt == "" {
		return -1
	}
	for i, existing := range list {
		if strings.TrimSpace(existing.CreatedAt) == createdAt && sameActor(existing.Author, comment.Author) {
			return i
		}
	}
	return -1
}

func removeComment(list []events.CommentInfo, comment events.CommentInfo) []events.CommentInfo {
	if len(list) == 0 {
		return nil
	}
	comments.EnsureID(&comment)
	if idx := findCommentIndex(list, comment); idx >= 0 {
		out := append([]events.CommentInfo(nil), list[:idx]...)
		out = append(out, list[idx+1:]...)
		if len(out) == 0 {
			return nil
		}
		return out
	}

	body := strings.TrimSpace(templates.StripHTML(comment.Body))
	for i := len(list) - 1; i >= 0; i-- {
		existing := list[i]
		if comment.Author != (events.Actor{}) && !sameActor(existing.Author, comment.Author) {
			continue
		}
		if body != "" && strings.TrimSpace(templates.StripHTML(existing.Body)) != body {
			continue
		}
		out := append([]events.CommentInfo(nil), list[:i]...)
		out = append(out, list[i+1:]...)
		if len(out) == 0 {
			return nil
		}
		return out
	}
	return list
}

func isEditComment(action string) bool {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "edited", "edit", "updated", "update", "modified", "modify":
		return true
	default:
		return false
	}
}

func isDeleteComment(action string) bool {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "deleted", "delete", "removed", "remove", "destroy":
		return true
	default:
		return false
	}
}

func shouldHandlePRComment(action string) bool {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "", "created", "create",
		"edited", "edit", "updated", "update", "modified", "modify",
		"deleted", "delete", "removed", "remove", "destroy":
		return true
	default:
		return false
	}
}
