package handlers

import (
	"testing"

	"gitverse-notifier/pkg/events"

	"github.com/stretchr/testify/assert"
)

func TestShouldNotifyPRComment(t *testing.T) {
	t.Parallel()

	assert.True(t, shouldNotifyPRComment(""))
	assert.True(t, shouldNotifyPRComment("created"))
	assert.True(t, shouldNotifyPRComment("Created"))
	assert.True(t, shouldNotifyPRComment("edited"))
	assert.False(t, shouldNotifyPRComment("deleted"))
	assert.False(t, shouldNotifyPRComment("delete"))
}

func TestTelegramThreadPlan(t *testing.T) {
	t.Parallel()

	replyTo, bind := telegramThreadPlan(events.PullRequestOpened, 0)
	assert.Equal(t, 0, replyTo)
	assert.True(t, bind)

	replyTo, bind = telegramThreadPlan(events.PullRequestOpened, 99)
	assert.Equal(t, 0, replyTo)
	assert.True(t, bind)

	replyTo, bind = telegramThreadPlan(events.PullRequestComment, 99)
	assert.Equal(t, 99, replyTo)
	assert.False(t, bind)

	replyTo, bind = telegramThreadPlan(events.PullRequestComment, 0)
	assert.Equal(t, 0, replyTo)
	assert.True(t, bind)

	replyTo, bind = telegramThreadPlan(events.PullRequestReviewRequested, 99)
	assert.Equal(t, 99, replyTo)
	assert.False(t, bind)

	replyTo, bind = telegramThreadPlan(events.BranchPush, 99)
	assert.Equal(t, 0, replyTo)
	assert.False(t, bind)
}
