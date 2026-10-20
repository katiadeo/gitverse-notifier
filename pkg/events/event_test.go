package events

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPullRequestInfoEffectiveNumber(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 412, PullRequestInfo{Number: 412, URL: "https://git.example/org/app/pulls/1"}.EffectiveNumber())
	assert.Equal(t, 218, PullRequestInfo{URL: "https://git.example/org/app/pulls/218"}.EffectiveNumber())
	assert.Equal(t, 15, PullRequestInfo{URL: "https://git.example/org/app/issues/15#comment"}.EffectiveNumber())
	assert.Equal(t, 0, PullRequestInfo{}.EffectiveNumber())
}

func TestResolveEventType_ReviewRequestedAliases(t *testing.T) {
	t.Parallel()

	cases := []struct {
		event     string
		eventType string
		action    string
		want      EventType
	}{
		{event: "pull_request", eventType: "pull_request", action: "review_requested", want: PullRequestReviewRequested},
		{event: "pull_request", eventType: "pull_request", action: "review_request", want: PullRequestReviewRequested},
		{event: "pull_request", eventType: "pull_request_review_request", action: "", want: PullRequestReviewRequested},
		{event: "pull_request", eventType: "pull_request_review_requested", action: "created", want: PullRequestReviewRequested},
		{event: "pull_request", eventType: "review_requested", action: "", want: PullRequestReviewRequested},
		{event: "pull_request", eventType: "review_request", action: "", want: PullRequestReviewRequested},
		{event: "pull_request", eventType: "pull_request", action: "review_request_removed", want: PullRequestReviewRequestRemoved},
		{event: "pull_request", eventType: "review_request_removed", action: "", want: PullRequestReviewRequestRemoved},
		{event: "pull_request", eventType: "pull_request_review_request_removed", action: "", want: PullRequestReviewRequestRemoved},
		{event: "pull_request", eventType: "pull_request_review_requested", action: "review_request_removed", want: PullRequestReviewRequestRemoved},
		{event: "pull_request", eventType: "pull_request_review_requested", action: "review_requested", want: PullRequestReviewRequested},
	}

	for _, tc := range cases {
		t.Run(tc.event+"."+tc.eventType+"."+tc.action, func(t *testing.T) {
			t.Parallel()
			got, err := ResolveEventType(tc.event, tc.eventType, tc.action)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
