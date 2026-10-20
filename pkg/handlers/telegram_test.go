package handlers

import (
	"testing"

	"gitverse-notifier/pkg/cache"
	"gitverse-notifier/pkg/config"
	"gitverse-notifier/pkg/events"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShouldHandlePRComment(t *testing.T) {
	t.Parallel()

	assert.True(t, shouldHandlePRComment(""))
	assert.True(t, shouldHandlePRComment("created"))
	assert.True(t, shouldHandlePRComment("Created"))
	assert.True(t, shouldHandlePRComment("edited"))
	assert.True(t, shouldHandlePRComment("updated"))
	assert.True(t, shouldHandlePRComment("deleted"))
	assert.True(t, shouldHandlePRComment("delete"))
	assert.True(t, shouldHandlePRComment("removed"))
	assert.False(t, shouldHandlePRComment("unknown"))
}

func TestAppendCommentKeepsSameAuthorTwice(t *testing.T) {
	t.Parallel()

	alice := events.Actor{Name: "Alice", Login: "alice", TgTag: "alice"}
	bob := events.Actor{Name: "bob", Login: "bob"}

	got := appendComment(nil, events.CommentInfo{ID: 1, Author: alice, Body: "first"}, "created")
	got = appendComment(got, events.CommentInfo{ID: 2, Author: bob, Body: "second"}, "created")
	got = appendComment(got, events.CommentInfo{ID: 3, Author: alice, Body: "third"}, "created")
	require.Len(t, got, 3)
	assert.Equal(t, "third", got[2].Body)

	got = appendComment(got, events.CommentInfo{ID: 2, Author: bob, Body: "second edited"}, "edited")
	require.Len(t, got, 3)
	assert.Equal(t, "second edited", got[1].Body)
	assert.True(t, got[1].Edited)
	assert.False(t, got[0].Edited)

	got = appendComment(nil, events.CommentInfo{ID: 9, Author: alice, Body: "link", URL: "https://git.example/pulls/1#issuecomment-9"}, "created")
	got = appendComment(got, events.CommentInfo{ID: 9, Author: alice, Body: "link edited"}, "edited")
	require.Equal(t, "https://git.example/pulls/1#issuecomment-9", got[0].URL)

	got = appendComment(nil, events.CommentInfo{
		Author: events.Actor{Login: "alice"},
		Body:   "first",
		URL:    "https://git.example/pulls/1#issuecomment-42",
	}, "created")
	got = appendComment(got, events.CommentInfo{
		ID:     42,
		Author: events.Actor{Name: "Alice", TgTag: "alice"},
		Body:   "first edited",
	}, "edited")
	require.Len(t, got, 1)
	assert.Equal(t, "first edited", got[0].Body)
	assert.True(t, got[0].Edited)
	assert.Equal(t, 42, got[0].ID)

	got = appendComment(got, events.CommentInfo{
		ID:     42,
		Author: events.Actor{Login: "alice"},
		Body:   "first again",
	}, "created")
	require.Len(t, got, 1)
	assert.Equal(t, "first again", got[0].Body)
}

func TestAppendCommentEditUsesCreatedAtWhenGitVerseOmitsID(t *testing.T) {
	t.Parallel()

	alice := events.Actor{Login: "alice", TgTag: "alice"}
	comments := []events.CommentInfo{
		{Author: alice, Body: "first", CreatedAt: "2026-10-20T17:03:29Z"},
		{Author: alice, Body: "second", CreatedAt: "2026-10-20T17:04:29Z"},
	}

	got := appendComment(comments, events.CommentInfo{
		Author:    alice,
		Body:      "first edited",
		CreatedAt: "2026-10-20T17:03:29Z",
	}, "edited")

	require.Len(t, got, 2)
	assert.Equal(t, "first edited", got[0].Body)
	assert.True(t, got[0].Edited)
	assert.Equal(t, "second", got[1].Body)
}

func TestAppendCommentEditUsesPreviousBodyForLegacyCard(t *testing.T) {
	t.Parallel()

	alice := events.Actor{Login: "alice", TgTag: "alice"}
	got := appendComment([]events.CommentInfo{
		{Author: alice, Body: "old comment"},
		{Author: alice, Body: "another comment"},
	}, events.CommentInfo{
		Author:       alice,
		Body:         "edited comment",
		PreviousBody: "old comment",
	}, "edited")

	require.Len(t, got, 2)
	assert.Equal(t, "edited comment", got[0].Body)
	assert.True(t, got[0].Edited)
	assert.Equal(t, "another comment", got[1].Body)
}

func TestRemoveCommentByIDAndBody(t *testing.T) {
	t.Parallel()

	alice := events.Actor{Name: "Alice", Login: "alice", TgTag: "alice"}
	bob := events.Actor{Name: "bob", Login: "bob", TgTag: "bob"}

	got := []events.CommentInfo{
		{ID: 1, Author: alice, Body: "first"},
		{ID: 2, Author: bob, Body: "second"},
		{ID: 3, Author: alice, Body: "third"},
	}
	got = removeComment(got, events.CommentInfo{ID: 2, Author: bob})
	require.Len(t, got, 2)
	assert.Equal(t, 1, got[0].ID)
	assert.Equal(t, 3, got[1].ID)

	got = removeComment(got, events.CommentInfo{Author: alice, Body: "third"})
	require.Len(t, got, 1)
	assert.Equal(t, "first", got[0].Body)

	got = removeComment(got, events.CommentInfo{ID: 1})
	require.Empty(t, got)
}

func TestApplyPRCardEvent_DeletesComment(t *testing.T) {
	t.Parallel()

	card := &cache.TelegramPRCard{
		Comments: []events.CommentInfo{
			{ID: 1, Author: events.Actor{TgTag: "alice"}, Body: "keep"},
			{ID: 2, Author: events.Actor{TgTag: "bob"}, Body: "drop"},
		},
	}
	applyPRCardEvent(card, &events.Event{
		Type:   events.PullRequestComment,
		Action: "deleted",
		Comment: events.CommentInfo{
			ID:     2,
			Author: events.Actor{TgTag: "bob"},
			Body:   "drop",
		},
	})
	require.Len(t, card.Comments, 1)
	assert.Equal(t, "keep", card.Comments[0].Body)

	applyPRCardEvent(card, &events.Event{
		Type:   events.PullRequestComment,
		Action: "deleted",
		Comment: events.CommentInfo{
			Author: events.Actor{TgTag: "alice"},
		},
	})
	require.Empty(t, card.Comments)
}

func TestApplyPRCardEvent_EditsComment(t *testing.T) {
	t.Parallel()

	card := &cache.TelegramPRCard{
		Comments: []events.CommentInfo{
			{ID: 7, Author: events.Actor{Login: "alice", TgTag: "alice"}, Body: "old"},
		},
	}
	applyPRCardEvent(card, &events.Event{
		Type:   events.PullRequestComment,
		Action: "edited",
		Comment: events.CommentInfo{
			ID:     7,
			Author: events.Actor{Login: "alice"},
			Body:   "new text",
		},
	})
	require.Len(t, card.Comments, 1)
	assert.Equal(t, "new text", card.Comments[0].Body)
	assert.True(t, card.Comments[0].Edited)
	assert.Equal(t, "alice", card.Comments[0].Author.TgTag)

	card = &cache.TelegramPRCard{
		Comments: []events.CommentInfo{
			{Author: events.Actor{Name: "kvolkov", TgTag: "screwpassenger"}, Body: "old", URL: "https://git.example/pulls/1#issuecomment-9"},
		},
	}
	applyPRCardEvent(card, &events.Event{
		Type:   events.PullRequestComment,
		Action: "edited",
		Comment: events.CommentInfo{
			ID:     9,
			Author: events.Actor{Login: "kvolkov"},
			Body:   "edited body",
		},
	})
	require.Len(t, card.Comments, 1)
	assert.Equal(t, "edited body", card.Comments[0].Body)
	assert.True(t, card.Comments[0].Edited)
	assert.Equal(t, "screwpassenger", card.Comments[0].Author.TgTag)
}

func TestApplyPRCardEvent_MergedByOnly(t *testing.T) {
	t.Parallel()

	card := &cache.TelegramPRCard{}
	applyPRCardEvent(card, &events.Event{
		Type:     events.PullRequestClosed,
		MergedBy: events.Actor{TgTag: "eve"},
		PullRequest: events.PullRequestInfo{
			State: "closed",
		},
	})
	require.True(t, card.Merged)
	require.Equal(t, "eve", card.MergedBy.TgTag)
}

func TestApplyPRCardEvent_ReviewDecisionReplacesPreviousOne(t *testing.T) {
	t.Parallel()

	card := &cache.TelegramPRCard{}
	reviewer := events.Actor{Login: "alice", TgTag: "alice"}
	applyPRCardEvent(card, &events.Event{Type: events.PullRequestReviewRejected, Sender: reviewer})
	require.Len(t, card.Rejections, 1)
	require.Empty(t, card.Approvals)

	applyPRCardEvent(card, &events.Event{Type: events.PullRequestReviewApproved, Sender: reviewer})
	require.Len(t, card.Approvals, 1)
	require.Empty(t, card.Rejections)

	applyPRCardEvent(card, &events.Event{Type: events.PullRequestReviewRejected, Sender: reviewer})
	require.Len(t, card.Rejections, 1)
	require.Empty(t, card.Approvals)
}

func TestIsPRCardEvent(t *testing.T) {
	t.Parallel()

	assert.True(t, isPRCardEvent(events.PullRequestOpened))
	assert.True(t, isPRCardEvent(events.PullRequestEdited))
	assert.True(t, isPRCardEvent(events.PullRequestComment))
	assert.True(t, isPRCardEvent(events.PullRequestReviewApproved))
	assert.True(t, isPRCardEvent(events.PullRequestReviewRequested))
	assert.True(t, isPRCardEvent(events.PullRequestReviewRequestRemoved))
	assert.True(t, isPRCardEvent(events.PullRequestClosed))
	assert.False(t, isPRCardEvent(events.BranchPush))
}

func TestApplyPRCardEvent(t *testing.T) {
	t.Parallel()

	card := &cache.TelegramPRCard{}
	applyPRCardEvent(card, &events.Event{
		Type:        events.PullRequestOpened,
		PullRequest: events.PullRequestInfo{Author: events.Actor{TgTag: "alice"}},
	})
	applyPRCardEvent(card, &events.Event{
		Type:    events.PullRequestComment,
		Comment: events.CommentInfo{Author: events.Actor{TgTag: "bob"}, Body: "nits"},
	})
	applyPRCardEvent(card, &events.Event{
		Type:   events.PullRequestReviewApproved,
		Sender: events.Actor{TgTag: "carol"},
	})
	applyPRCardEvent(card, &events.Event{
		Type:        events.PullRequestClosed,
		Sender:      events.Actor{TgTag: "dave"},
		PullRequest: events.PullRequestInfo{Merged: true},
	})

	require.True(t, card.Opened)
	require.True(t, card.Merged)
	require.Equal(t, "bob", card.Comments[0].Author.TgTag)
	require.Equal(t, "carol", card.Approvals[0].Author.TgTag)
	require.Equal(t, "dave", card.MergedBy.TgTag)
}

func TestApplyPRCardEvent_ReviewersAddRemoveAndPersist(t *testing.T) {
	t.Parallel()

	alice := events.Actor{ID: 1, Login: "alice", TgTag: "alice"}
	bob := events.Actor{ID: 2, Login: "bob", TgTag: "bob"}
	carol := events.Actor{ID: 3, Login: "carol", TgTag: "carol"}

	card := &cache.TelegramPRCard{}
	applyPRCardEvent(card, &events.Event{
		Type: events.PullRequestOpened,
		PullRequest: events.PullRequestInfo{
			Reviewers:      []events.Actor{alice, bob},
			ReviewersExact: true,
		},
	})
	require.Equal(t, []events.Actor{alice, bob}, card.Reviewers)

	applyPRCardEvent(card, &events.Event{
		Type:    events.PullRequestComment,
		Comment: events.CommentInfo{Author: events.Actor{TgTag: "dave"}, Body: "nits"},
	})
	require.Equal(t, []events.Actor{alice, bob}, card.Reviewers)

	event := &events.Event{Type: events.PullRequestComment}
	applyCardToEvent(event, card)
	require.Equal(t, []events.Actor{alice, bob}, event.PullRequest.Reviewers)

	applyPRCardEvent(card, &events.Event{
		Type: events.PullRequestReviewRequested,
		PullRequest: events.PullRequestInfo{
			Reviewers: []events.Actor{carol},
		},
	})
	require.Equal(t, []events.Actor{alice, bob, carol}, card.Reviewers)

	applyPRCardEvent(card, &events.Event{
		Type: events.PullRequestReviewRequestRemoved,
		PullRequest: events.PullRequestInfo{
			Reviewers: []events.Actor{{Login: "bob"}},
		},
	})
	require.Equal(t, []events.Actor{alice, carol}, card.Reviewers)

	applyPRCardEvent(card, &events.Event{
		Type: events.PullRequestEdited,
		PullRequest: events.PullRequestInfo{
			Reviewers:      []events.Actor{alice},
			ReviewersExact: true,
		},
	})
	require.Equal(t, []events.Actor{alice}, card.Reviewers)

	applyPRCardEvent(card, &events.Event{
		Type: events.PullRequestReviewRequestRemoved,
		PullRequest: events.PullRequestInfo{
			Reviewers:      nil,
			ReviewersExact: true,
		},
	})
	require.Empty(t, card.Reviewers)
}

func TestApplyPRCardEvent_CommentKeepsAPIReviewers(t *testing.T) {
	t.Parallel()

	card := &cache.TelegramPRCard{Opened: true}
	applyPRCardEvent(card, &events.Event{
		Type: events.PullRequestComment,
		Comment: events.CommentInfo{
			Author: events.Actor{TgTag: "alice"},
			Body:   "nits",
		},
		PullRequest: events.PullRequestInfo{
			Reviewers:      []events.Actor{{Login: "bob", TgTag: "bob"}},
			ReviewersExact: true,
		},
	})
	require.Equal(t, "bob", card.Reviewers[0].TgTag)
}

func TestApplyPRCardEvent_CommentURLFromPR(t *testing.T) {
	t.Parallel()

	card := &cache.TelegramPRCard{URL: "https://git.example/org/app/pulls/412"}
	event := &events.Event{
		Type: events.PullRequestComment,
		Comment: events.CommentInfo{
			ID:     77,
			Author: events.Actor{TgTag: "alice"},
			Body:   "please check",
		},
	}
	applyPRCardEvent(card, event)
	require.Equal(t, "https://git.example/org/app/pulls/412#issuecomment-77", card.Comments[0].URL)
}

func TestPRCardRecoversNumberAndURLFromAnyIssueKey(t *testing.T) {
	config.GlobalConfig = &config.Config{GitverseBaseURL: "https://git.example"}
	threads := cache.NewTelegramPRThreads()
	threads.Put(&cache.TelegramPRCard{
		RootID:     10,
		Repository: "bks-frontend/fcs-frontend",
		Number:     214,
		Title:      "APP-1168, APP-1167 [FE] original",
	})
	a := &TelegramNotify{threads: threads}
	event := &events.Event{
		Repository: "bks-frontend/fcs-frontend",
		PullRequest: events.PullRequestInfo{
			Title: "APP-1167 , APP-1168 [FE] form",
		},
	}

	card := a.prCard(event)
	require.Equal(t, 214, event.PullRequest.Number)
	require.Equal(t, "https://git.example/bks-frontend/fcs-frontend/pulls/214", event.PullRequest.URL)
	require.Equal(t, 10, card.ResolvedMessageID())
}
