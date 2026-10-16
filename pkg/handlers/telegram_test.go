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

func TestAppendUniqueActor(t *testing.T) {
	t.Parallel()

	alice := events.Actor{Name: "Alice", Login: "alice", TgTag: "alice"}
	aliceAgain := events.Actor{Name: "alice git", Login: "alice", TgTag: "alice"}
	bob := events.Actor{Name: "bob", Login: "bob"}

	got, added := appendUniqueActor(nil, alice)
	assert.True(t, added)
	assert.Equal(t, []events.Actor{alice}, got)

	got, added = appendUniqueActor(got, aliceAgain)
	assert.False(t, added)

	got, added = appendUniqueActor(got, bob)
	assert.True(t, added)
	assert.Equal(t, []events.Actor{alice, bob}, got)
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

func TestAppendUniqueID(t *testing.T) {
	t.Parallel()

	got := appendUniqueID(nil, 0)
	assert.Nil(t, got)

	got = appendUniqueID(got, 10)
	assert.Equal(t, []int{10}, got)

	got = appendUniqueID(got, 10)
	assert.Equal(t, []int{10}, got)

	got = appendUniqueID(got, 11)
	assert.Equal(t, []int{10, 11}, got)
}
