package settings

import (
	"testing"

	"gitverse-notifier/pkg/events"

	"github.com/stretchr/testify/assert"
)

func TestActionRuleAllowed_SkipEmptyIgnoresDeletedComments(t *testing.T) {
	t.Parallel()

	rule := ActionRule{On: events.PullRequestComment, SkipEmpty: true}

	assert.False(t, rule.Allowed(&events.Event{
		Type:    events.PullRequestComment,
		Action:  "created",
		Comment: events.CommentInfo{Body: ""},
	}))
	assert.True(t, rule.Allowed(&events.Event{
		Type:    events.PullRequestComment,
		Action:  "deleted",
		Comment: events.CommentInfo{Body: ""},
	}))
	assert.True(t, rule.Allowed(&events.Event{
		Type:    events.PullRequestComment,
		Action:  "removed",
		Comment: events.CommentInfo{Body: ""},
	}))
	assert.True(t, rule.Allowed(&events.Event{
		Type:    events.PullRequestComment,
		Action:  "edited",
		Comment: events.CommentInfo{Body: "updated"},
	}))
}
