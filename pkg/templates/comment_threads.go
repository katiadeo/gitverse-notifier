package templates

import (
	"gitverse-notifier/pkg/comments"
	"gitverse-notifier/pkg/events"
)

type CommentThread struct {
	Root    events.CommentInfo
	Replies []events.CommentInfo
}

func CommentThreads(list []events.CommentInfo) []CommentThread {
	if len(list) == 0 {
		return nil
	}
	byID := make(map[int]events.CommentInfo, len(list))
	for _, c := range list {
		if id := comments.Identity(c); id > 0 {
			byID[id] = c
		}
	}

	type bucket struct {
		items []events.CommentInfo
	}
	buckets := make([]*bucket, 0)
	index := map[int]*bucket{}
	nextOrphan := -1

	for _, c := range list {
		key := threadRootID(c, byID)
		if key <= 0 {
			key = nextOrphan
			nextOrphan--
		}
		b := index[key]
		if b == nil {
			b = &bucket{}
			index[key] = b
			buckets = append(buckets, b)
		}
		b.items = append(b.items, c)
	}

	out := make([]CommentThread, 0, len(buckets))
	for _, b := range buckets {
		out = append(out, splitCommentThread(b.items))
	}

	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func splitCommentThread(items []events.CommentInfo) CommentThread {
	if len(items) == 0 {
		return CommentThread{}
	}
	rootIdx := -1
	for i, item := range items {
		if item.ReplyTo == 0 {
			rootIdx = i
			break
		}
	}
	if rootIdx < 0 {
		rootIdx = 0
	}
	thread := CommentThread{Root: items[rootIdx]}
	for i, item := range items {
		if i == rootIdx {
			continue
		}
		thread.Replies = append(thread.Replies, item)
	}
	return thread
}

func threadRootID(c events.CommentInfo, byID map[int]events.CommentInfo) int {
	if c.ThreadID > 0 {
		return c.ThreadID
	}
	id := comments.Identity(c)
	if c.ReplyTo <= 0 {
		return id
	}
	seen := map[int]bool{}
	cur := c.ReplyTo
	for cur > 0 && !seen[cur] {
		seen[cur] = true
		parent, ok := byID[cur]
		if !ok {
			return cur
		}
		if parent.ThreadID > 0 {
			return parent.ThreadID
		}
		if parent.ReplyTo <= 0 {
			if pid := comments.Identity(parent); pid > 0 {
				return pid
			}
			return cur
		}
		cur = parent.ReplyTo
	}
	if id > 0 {
		return id
	}
	return c.ReplyTo
}
