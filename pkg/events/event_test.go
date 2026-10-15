package events

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveEventType_ReviewRequestedAliases(t *testing.T) {
	t.Parallel()

	cases := []struct {
		event     string
		eventType string
		action    string
	}{
		{event: "pull_request", eventType: "pull_request", action: "review_requested"},
		{event: "pull_request", eventType: "pull_request", action: "review_request"},
		{event: "pull_request", eventType: "pull_request_review_request", action: ""},
		{event: "pull_request", eventType: "pull_request_review_requested", action: "created"},
		{event: "pull_request", eventType: "review_requested", action: ""},
		{event: "pull_request", eventType: "review_request", action: ""},
	}

	for _, tc := range cases {
		t.Run(tc.event+"."+tc.eventType+"."+tc.action, func(t *testing.T) {
			t.Parallel()
			got, err := ResolveEventType(tc.event, tc.eventType, tc.action)
			require.NoError(t, err)
			assert.Equal(t, PullRequestReviewRequested, got)
		})
	}
}
