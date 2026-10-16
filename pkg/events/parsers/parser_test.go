package parsers

import (
	"context"
	"encoding/json"
	"testing"

	"gitverse-notifier/pkg/events"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return data
}

func TestParseEvent_PullRequestOpened(t *testing.T) {
	body := mustJSON(t, map[string]any{
		"action": "opened",
		"number": 42,
		"repository": map[string]any{
			"fullName": "org/app",
		},
		"sender": map[string]any{
			"id":    7,
			"name":  "alice",
			"email": "alice@example.com",
		},
		"pullRequest": map[string]any{
			"title": "Add feature",
			"body":  "Implements APP-1",
			"state": "open",
			"user": map[string]any{
				"id":   8,
				"name": "bob",
			},
			"head": map[string]any{
				"ref": "APP-1",
			},
			"base": map[string]any{
				"ref": "develop",
			},
		},
	})

	event, err := ParseEvent(context.Background(), "pull_request", "pull_request", "123", body)
	require.NoError(t, err)

	assert.Equal(t, events.PullRequestOpened, event.Type)
	assert.Equal(t, "opened", event.Action)
	assert.Equal(t, "org/app", event.Repository)
	assert.Equal(t, events.Actor{ID: 7, Name: "alice", Login: "alice", Email: "alice@example.com"}, event.Sender)
	assert.Equal(t, 42, event.PullRequest.Number)
	assert.Equal(t, "Add feature", event.PullRequest.Title)
	assert.Equal(t, "Implements APP-1", event.PullRequest.Body)
	assert.Equal(t, "open", event.PullRequest.State)
	assert.Equal(t, events.Actor{ID: 8, Name: "bob", Login: "bob"}, event.PullRequest.Author)
	assert.Equal(t, "APP-1", event.PullRequest.HeadRef)
	assert.Equal(t, "develop", event.PullRequest.BaseRef)
	assert.Empty(t, event.Comment.Body)
}

func TestParseEvent_PullRequestReviewComment(t *testing.T) {
	body := mustJSON(t, map[string]any{
		"repository": map[string]any{
			"full_name": "org/app",
		},
		"sender": map[string]any{
			"name": "reviewer",
		},
		"number": 10,
		"pullRequest": map[string]any{
			"title": "Review me",
			"state": "open",
		},
		"review": map[string]any{
			"type":    "comment",
			"content": "Looks good",
		},
	})

	event, err := ParseEvent(context.Background(), "pull_request_comment", "pull_request_review_comment", "123", body)
	require.NoError(t, err)

	assert.Equal(t, events.PullRequestReviewComment, event.Type)
	assert.Equal(t, "org/app", event.Repository)
	assert.Equal(t, "Looks good", event.Comment.Body)
	assert.Equal(t, "reviewer", event.Comment.Author.Name)
	assert.Equal(t, "Review me", event.PullRequest.Title)
}

func TestParseEvent_PullRequestReviewComment_EmptyContentStillSetsComment(t *testing.T) {
	body := mustJSON(t, map[string]any{
		"repository": map[string]any{"fullName": "org/app"},
		"sender":     map[string]any{"name": "reviewer"},
		"number":     11,
		"pullRequest": map[string]any{
			"title": "Empty review",
		},
		"review": map[string]any{
			"content": "",
		},
	})

	event, err := ParseEvent(context.Background(), "pull_request_comment", "pull_request_review_comment", "123", body)
	require.NoError(t, err)
	assert.Equal(t, events.PullRequestReviewComment, event.Type)
	assert.Equal(t, "", event.Comment.Body)
	assert.Equal(t, "reviewer", event.Comment.Author.Name)
}

func TestParseEvent_IssueComment(t *testing.T) {
	body := mustJSON(t, map[string]any{
		"repository": map[string]any{"fullName": "org/app"},
		"sender":     map[string]any{"name": "carol"},
		"isPull":     true,
		"number":     738,
		"issue": map[string]any{
			"number":   738,
			"title":    "PR title",
			"body":     "PR body",
			"state":    "open",
			"html_url": "https://git.example/org/app/pulls/738",
			"user":     map[string]any{"name": "author"},
		},
		"comment": map[string]any{
			"body": "Please fix",
			"user": map[string]any{"name": "commenter"},
		},
	})

	event, err := ParseEvent(context.Background(), "issue_comment", "pull_request_comment", "123", body)
	require.NoError(t, err)

	assert.Equal(t, events.PullRequestComment, event.Type)
	assert.Equal(t, "Please fix", event.Comment.Body)
	assert.Equal(t, "commenter", event.Comment.Author.Name)
	assert.Equal(t, "PR title", event.PullRequest.Title)
	assert.Equal(t, "author", event.PullRequest.Author.Name)
	assert.Equal(t, 738, event.PullRequest.Number)
	assert.Equal(t, "https://git.example/org/app/pulls/738", event.PullRequest.URL)
}

func TestParseEvent_IssueComment_GitverseSnakeCasePayload(t *testing.T) {
	body := mustJSON(t, map[string]any{
		"action":  "created",
		"is_pull": true,
		"repository": map[string]any{
			"full_name": "org/app",
		},
		"sender": map[string]any{"login": "alice"},
		"issue": map[string]any{
			"number":   237,
			"title":    "APP-738 generator",
			"html_url": "https://git.example/org/app/pulls/237",
			"user":     map[string]any{"login": "alice"},
		},
		"pull_request": map[string]any{
			"number":   237,
			"html_url": "https://git.example/org/app/pulls/237",
			"head":     map[string]any{"ref": "APP-738-generator"},
			"base":     map[string]any{"ref": "develop"},
		},
		"comment": map[string]any{
			"body":             "super final check",
			"pull_request_url": "https://git.example/org/app/pulls/237",
			"user":             map[string]any{"login": "alice"},
		},
	})

	event, err := ParseEvent(context.Background(), "issue_comment", "pull_request_comment", "123", body)
	require.NoError(t, err)
	assert.Equal(t, 237, event.PullRequest.Number)
	assert.Equal(t, "https://git.example/org/app/pulls/237", event.PullRequest.URL)
	assert.Equal(t, "alice", event.Comment.Author.Name)
	assert.Equal(t, "APP-738-generator", event.PullRequest.HeadRef)
	assert.Equal(t, "develop", event.PullRequest.BaseRef)
	assert.Equal(t, events.PullRequestComment, event.Type)
}

func TestParseEvent_PullRequestOpened_GitverseSnakeCase(t *testing.T) {
	body := mustJSON(t, map[string]any{
		"action": "opened",
		"number": 237,
		"repository": map[string]any{
			"full_name": "org/app",
		},
		"sender": map[string]any{
			"login":    "alice",
			"html_url": "https://git.example/alice",
		},
		"pull_request": map[string]any{
			"number":   237,
			"title":    "APP-738 generator",
			"html_url": "https://git.example/org/app/pulls/237",
			"user":     map[string]any{"login": "alice"},
			"head":     map[string]any{"ref": "APP-738-generator"},
			"base":     map[string]any{"ref": "develop"},
		},
	})

	event, err := ParseEvent(context.Background(), "pull_request", "pull_request", "123", body)
	require.NoError(t, err)
	assert.Equal(t, events.PullRequestOpened, event.Type)
	assert.Equal(t, 237, event.PullRequest.Number)
	assert.Equal(t, "https://git.example/org/app/pulls/237", event.PullRequest.URL)
	assert.Equal(t, "alice", event.Sender.Name)
	assert.Equal(t, "https://git.example/alice", event.Sender.URL)
	assert.Equal(t, "APP-738-generator", event.PullRequest.HeadRef)
}

func TestParseEvent_IssueComment_NumberFromCommentHTMLURL(t *testing.T) {
	body := mustJSON(t, map[string]any{
		"action":     "created",
		"is_pull":    true,
		"repository": map[string]any{"full_name": "org/app"},
		"sender":     map[string]any{"login": "alice"},
		"issue": map[string]any{
			"title": "APP-1183 [FE] Доработать форму",
			"user":  map[string]any{"login": "bob", "html_url": "https://git.example/bob"},
		},
		"comment": map[string]any{
			"body":     "Тогда `z.array(z.string())` @bob",
			"html_url": "https://git.example/org/app/pulls/412#issuecomment-1",
			"user":     map[string]any{"login": "alice"},
		},
	})

	event, err := ParseEvent(context.Background(), "issue_comment", "pull_request_comment", "123", body)
	require.NoError(t, err)
	assert.Equal(t, 412, event.PullRequest.Number)
	assert.Equal(t, "https://git.example/org/app/pulls/412", event.PullRequest.URL)
	assert.Equal(t, "bob", event.PullRequest.Author.Login)
	require.Len(t, event.Comment.Mentions, 1)
	assert.Equal(t, "bob", event.Comment.Mentions[0].Login)
}

func TestParseEvent_IssueComment_NumberFromAPIIssueURL(t *testing.T) {
	body := mustJSON(t, map[string]any{
		"action":     "created",
		"is_pull":    true,
		"repository": map[string]any{"full_name": "org/app"},
		"sender":     map[string]any{"login": "alice"},
		"issue": map[string]any{
			"title": "APP-1184 [FE] Доработка формы",
			"url":   "https://git.example/api/v1/repos/org/app/issues/390",
			"user":  map[string]any{"login": "bob"},
		},
		"comment": map[string]any{
			"body": "test",
			"user": map[string]any{"login": "alice"},
		},
	})

	event, err := ParseEvent(context.Background(), "issue_comment", "pull_request_comment", "123", body)
	require.NoError(t, err)
	assert.Equal(t, 390, event.PullRequest.Number)
	assert.Equal(t, "https://git.example/org/app/pulls/390", event.PullRequest.URL)
}

func TestParseEvent_IssueComment_NumberFromGitAPIIssueURL(t *testing.T) {
	body := mustJSON(t, map[string]any{
		"action":     "edited",
		"is_pull":    true,
		"repository": map[string]any{"full_name": "org/app"},
		"sender":     map[string]any{"login": "alice"},
		"issue": map[string]any{
			"title": "APP-1184 [FE] form",
			"user":  map[string]any{"login": "bob"},
		},
		"comment": map[string]any{
			"body":      "test test_code test_update_comment",
			"issue_url": "https://git-api.example/repos/org/app/issues/390",
			"user":      map[string]any{"login": "alice"},
		},
	})

	event, err := ParseEvent(context.Background(), "issue_comment", "pull_request_comment", "123", body)
	require.NoError(t, err)
	assert.Equal(t, 390, event.PullRequest.Number)
	assert.Equal(t, "https://git-api.example/org/app/pulls/390", event.PullRequest.URL)
	assert.Equal(t, "edited", event.Action)
}

func TestParseEvent_IssueComment_NumberFromCamelCaseHTMLUrl(t *testing.T) {
	body := mustJSON(t, map[string]any{
		"action":  "edited",
		"is_pull": true,
		"repository": map[string]any{
			"name":  "app",
			"owner": map[string]any{"login": "org"},
		},
		"sender": map[string]any{"login": "alice"},
		"issue": map[string]any{
			"title": "APP-1184 [FE] form",
			"user":  map[string]any{"login": "bob"},
		},
		"comment": map[string]any{
			"body":    "APP-1184",
			"htmlUrl": "https://git.example/org/app/pulls/390#issuecomment-1",
			"user":    map[string]any{"login": "alice"},
		},
	})

	event, err := ParseEvent(context.Background(), "issue_comment", "pull_request_comment", "123", body)
	require.NoError(t, err)
	assert.Equal(t, 390, event.PullRequest.Number)
	assert.Equal(t, "org/app", event.Repository)
	assert.Equal(t, "https://git.example/org/app/pulls/390", event.PullRequest.URL)
}

func TestParseEvent_IssueComment_NumberFromNestedIndexAndDraftTitle(t *testing.T) {
	body := mustJSON(t, map[string]any{
		"action":  "edited",
		"is_pull": true,
		"repo": map[string]any{
			"name":  "app",
			"owner": map[string]any{"username": "org"},
		},
		"issue": map[string]any{
			"title": "Draft:  APP-1197 [FE] form",
			"index": "241",
			"user":  map[string]any{"login": "bob"},
		},
		"comment": map[string]any{
			"body": "Testing gitverse bot",
			"user": map[string]any{"login": "alice"},
		},
	})

	event, err := ParseEvent(context.Background(), "issue_comment", "pull_request_comment", "123", body)
	require.NoError(t, err)
	assert.Equal(t, 241, event.PullRequest.Number)
	assert.Equal(t, "org/app", event.Repository)
	assert.Equal(t, "Draft:  APP-1197 [FE] form", event.PullRequest.Title)
}

func TestParseEvent_IssueComment_NumberFromMergeRequestURL(t *testing.T) {
	body := mustJSON(t, map[string]any{
		"action":     "created",
		"is_pull":    true,
		"repository": map[string]any{"full_name": "org/app"},
		"issue":      map[string]any{"title": "APP-1"},
		"comment": map[string]any{
			"body": "x",
			"url":  "https://git.example/org/app/merge-requests/88#note-1",
		},
	})

	event, err := ParseEvent(context.Background(), "issue_comment", "pull_request_comment", "123", body)
	require.NoError(t, err)
	assert.Equal(t, 88, event.PullRequest.Number)
}

func TestParseEvent_IssueComment_SparseGitversePayloadHasNoNumber(t *testing.T) {
	body := mustJSON(t, map[string]any{
		"action": "created",
		"issue": map[string]any{
			"user":  map[string]any{"id": 50, "name": "bob"},
			"title": "APP-1183 form",
			"body":  "desc",
			"state": "open",
		},
		"comment": map[string]any{
			"user": map[string]any{"id": 24, "name": "alice"},
			"body": "note",
		},
		"repository": map[string]any{
			"id":        45,
			"name":      "app",
			"ownerName": "org",
			"fullName":  "org/app",
		},
		"sender": map[string]any{"id": 24, "name": "alice"},
		"isPull": true,
	})

	event, err := ParseEvent(context.Background(), "issue_comment", "pull_request_comment", "123", body)
	require.NoError(t, err)
	assert.Equal(t, events.PullRequestComment, event.Type)
	assert.Equal(t, "org/app", event.Repository)
	assert.Equal(t, "APP-1183 form", event.PullRequest.Title)
	assert.Equal(t, 0, event.PullRequest.Number)
	assert.Empty(t, event.PullRequest.URL)
	assert.Equal(t, "alice", event.Comment.Author.Name)
	assert.Equal(t, "bob", event.PullRequest.Author.Name)
}

func TestParseEvent_IssueComment_NumberFromPullRequestIndex(t *testing.T) {
	body := mustJSON(t, map[string]any{
		"action":     "created",
		"is_pull":    true,
		"repository": map[string]any{"full_name": "org/app"},
		"issue": map[string]any{
			"title": "APP-1183 form",
			"user":  map[string]any{"username": "bob"},
		},
		"pull_request": map[string]any{
			"index":   412,
			"web_url": "/org/app/pulls/412",
		},
		"comment": map[string]any{
			"body": "x",
			"user": map[string]any{"username": "carol"},
		},
	})

	event, err := ParseEvent(context.Background(), "issue_comment", "pull_request_comment", "123", body)
	require.NoError(t, err)
	assert.Equal(t, 412, event.PullRequest.Number)
	assert.Equal(t, "/org/app/pulls/412", event.PullRequest.URL)
	assert.Equal(t, "org/app", event.Repository)
	assert.Equal(t, "carol", event.Comment.Author.Login)
	assert.Equal(t, "bob", event.PullRequest.Author.Login)
}

func TestParseEvent_IssueComment_NumberFromRelativeHTMLURL(t *testing.T) {
	body := mustJSON(t, map[string]any{
		"action":     "created",
		"is_pull":    true,
		"repository": map[string]any{"full_name": "org/app"},
		"issue":      map[string]any{"title": "APP-1183 form"},
		"comment": map[string]any{
			"body":     "x",
			"html_url": "/org/app/pulls/412#note-9",
		},
	})

	event, err := ParseEvent(context.Background(), "issue_comment", "pull_request_comment", "123", body)
	require.NoError(t, err)
	assert.Equal(t, 412, event.PullRequest.Number)
	assert.Equal(t, "/org/app/pulls/412", event.PullRequest.URL)
	assert.Equal(t, "org/app", event.Repository)
}

func TestParseEvent_IssueComment_NumberFromPullRequestObject(t *testing.T) {
	body := mustJSON(t, map[string]any{
		"repository":  map[string]any{"fullName": "org/app"},
		"sender":      map[string]any{"name": "carol"},
		"isPull":      true,
		"pullRequest": map[string]any{"number": 91, "title": "via PR object"},
		"issue":       map[string]any{"title": "via PR object"},
		"comment":     map[string]any{"body": "x", "user": map[string]any{"name": "c"}},
	})

	event, err := ParseEvent(context.Background(), "issue_comment", "pull_request_comment", "123", body)
	require.NoError(t, err)
	assert.Equal(t, 91, event.PullRequest.Number)
}

func TestParseEvent_IssueComment_FallsBackToSenderAuthor(t *testing.T) {
	body := mustJSON(t, map[string]any{
		"repository": map[string]any{"fullName": "org/app"},
		"sender":     map[string]any{"name": "carol"},
		"comment": map[string]any{
			"body": "no user",
		},
	})

	event, err := ParseEvent(context.Background(), "issue_comment", "pull_request_comment", "123", body)
	require.NoError(t, err)
	assert.Equal(t, "carol", event.Comment.Author.Name)
	assert.Empty(t, event.PullRequest.Title)
}

func TestParseEvent_BranchPush(t *testing.T) {
	body := mustJSON(t, map[string]any{
		"ref":          "refs/heads/main",
		"before":       "aaa",
		"after":        "bbb",
		"totalCommits": 3,
		"repository": map[string]any{
			"fullName": "org/app",
		},
		"pusher": map[string]any{
			"name": "dave",
		},
	})

	event, err := ParseEvent(context.Background(), "push", "push", "123", body)
	require.NoError(t, err)

	assert.Equal(t, events.BranchPush, event.Type)
	assert.Equal(t, "refs/heads/main", event.Ref)
	assert.Equal(t, "main", event.Branch)
	assert.Equal(t, "dave", event.Sender.Name)
	assert.Equal(t, events.PushInfo{Before: "aaa", After: "bbb", TotalCommits: 3}, event.Push)
}

func TestParseEvent_BranchCreatedAndDeleted(t *testing.T) {
	createdBody := mustJSON(t, map[string]any{
		"ref": "refs/heads/feature",
		"repository": map[string]any{
			"fullName": "org/app",
		},
		"sender": map[string]any{"name": "erin"},
	})
	created, err := ParseEvent(context.Background(), "create", "create", "123", createdBody)
	require.NoError(t, err)
	assert.Equal(t, events.BranchCreated, created.Type)
	assert.Equal(t, "feature", created.Branch)

	deletedBody := mustJSON(t, map[string]any{
		"ref": "refs/heads/old",
		"repository": map[string]any{
			"fullName": "org/app",
		},
		"sender": map[string]any{"name": "frank"},
	})
	deleted, err := ParseEvent(context.Background(), "delete", "delete", "123", deletedBody)
	require.NoError(t, err)
	assert.Equal(t, events.BranchDeleted, deleted.Type)
	assert.Equal(t, "old", deleted.Branch)
}

func TestParseEvent_CICDStatus(t *testing.T) {
	body := mustJSON(t, map[string]any{
		"state":       "success",
		"context":     "ci/tests",
		"description": "All good",
		"sha":         "deadbeef",
		"repository": map[string]any{
			"fullName": "org/app",
		},
		"sender": map[string]any{"name": "bot"},
	})

	event, err := ParseEvent(context.Background(), "status", "status", "123", body)
	require.NoError(t, err)

	assert.Equal(t, events.CICDStatus, event.Type)
	assert.Equal(t, events.StatusInfo{
		State:       "success",
		Context:     "ci/tests",
		Description: "All good",
		SHA:         "deadbeef",
	}, event.Status)
}

func TestParseEvent_PullRequestAuthorFallsBackToSender(t *testing.T) {
	body := mustJSON(t, map[string]any{
		"action": "closed",
		"number": 5,
		"repository": map[string]any{
			"fullName": "org/app",
		},
		"sender": map[string]any{
			"name": "alice",
		},
		"pullRequest": map[string]any{
			"title": "Close me",
			"state": "closed",
		},
	})

	event, err := ParseEvent(context.Background(), "pull_request", "pull_request", "123", body)
	require.NoError(t, err)
	assert.Equal(t, events.PullRequestClosed, event.Type)
	assert.Equal(t, "alice", event.PullRequest.Author.Name)
}

func TestParseEvent_ReviewRequested_GitverseSnakeCase(t *testing.T) {
	body := mustJSON(t, map[string]any{
		"action": "review_requested",
		"number": 281,
		"repository": map[string]any{
			"full_name": "org/app",
		},
		"sender": map[string]any{"login": "alice"},
		"requested_reviewer": map[string]any{
			"id":    38,
			"login": "alice",
		},
		"pull_request": map[string]any{
			"number":   281,
			"title":    "APP-281: deploy guide",
			"html_url": "https://git.example/org/app/pulls/281",
			"user":     map[string]any{"login": "carol"},
			"requested_reviewers": []any{
				map[string]any{"login": "dave"},
				map[string]any{"login": "alice"},
			},
		},
	})

	event, err := ParseEvent(context.Background(), "pull_request", "pull_request_review_requested", "123", body)
	require.NoError(t, err)
	assert.Equal(t, events.PullRequestReviewRequested, event.Type)
	assert.Equal(t, 281, event.PullRequest.Number)
	assert.Equal(t, "https://git.example/org/app/pulls/281", event.PullRequest.URL)
	require.Len(t, event.PullRequest.Reviewers, 1)
	assert.Equal(t, "alice", event.PullRequest.Reviewers[0].Name)
}

func TestParseEvent_IssueCommentDeletedKeepsAction(t *testing.T) {
	body := mustJSON(t, map[string]any{
		"action":     "deleted",
		"is_pull":    true,
		"repository": map[string]any{"full_name": "org/app"},
		"sender":     map[string]any{"login": "alice"},
		"issue": map[string]any{
			"number": 237,
			"title":  "PR title",
		},
		"comment": map[string]any{
			"body": "bla bla bla blaaa",
			"user": map[string]any{"login": "alice"},
		},
	})

	event, err := ParseEvent(context.Background(), "issue_comment", "pull_request_comment", "123", body)
	require.NoError(t, err)
	assert.Equal(t, events.PullRequestComment, event.Type)
	assert.Equal(t, "deleted", event.Action)
	assert.Equal(t, "bla bla bla blaaa", event.Comment.Body)
}

func TestParseEvent_UsesActionFromBodyForTypeResolution(t *testing.T) {
	cases := []struct {
		action string
		want   events.EventType
	}{
		{action: "opened", want: events.PullRequestOpened},
		{action: "edited", want: events.PullRequestEdited},
		{action: "synchronized", want: events.PullRequestSynchronized},
		{action: "review_requested", want: events.PullRequestReviewRequested},
	}

	for _, tc := range cases {
		t.Run(tc.action, func(t *testing.T) {
			body := mustJSON(t, map[string]any{
				"action": tc.action,
				"repository": map[string]any{
					"fullName": "org/app",
				},
				"sender": map[string]any{"name": "alice"},
				"pullRequest": map[string]any{
					"title": "x",
				},
			})

			event, err := ParseEvent(context.Background(), "pull_request", "pull_request", "123", body)
			require.NoError(t, err)
			assert.Equal(t, tc.want, event.Type)
		})
	}
}

func TestParseEvent_Errors(t *testing.T) {
	t.Run("invalid common json", func(t *testing.T) {
		_, err := ParseEvent(context.Background(), "push", "push", "123", []byte("{"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse event json")
	})

	t.Run("unknown event type", func(t *testing.T) {
		body := mustJSON(t, map[string]any{
			"repository": map[string]any{"fullName": "org/app"},
		})
		_, err := ParseEvent(context.Background(), "weird", "event", "123", body)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to detect event type")
	})

	t.Run("non-numeric pull request number is ignored", func(t *testing.T) {
		body := []byte(`{"action":"opened","repository":{"fullName":"org/app"},"number":"not-int","pullRequest":{"title":"x"}}`)
		event, err := ParseEvent(context.Background(), "pull_request", "pull_request", "123", body)
		require.NoError(t, err)
		assert.Equal(t, 0, event.PullRequest.Number)
		assert.Equal(t, "x", event.PullRequest.Title)
	})

	t.Run("invalid issue comment json", func(t *testing.T) {
		body := []byte(`{"repository":{"fullName":"org/app"},"isPull":"nope"}`)
		_, err := ParseEvent(context.Background(), "issue_comment", "pull_request_comment", "123", body)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse issue comment json")
	})

	t.Run("invalid push json", func(t *testing.T) {
		body := []byte(`{"repository":{"fullName":"org/app"},"totalCommits":"x"}`)
		_, err := ParseEvent(context.Background(), "push", "push", "123", body)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse push/ref json")
	})

	t.Run("invalid status json", func(t *testing.T) {
		body := []byte(`{"repository":{"fullName":"org/app"},"state":{"nested":true}}`)
		_, err := ParseEvent(context.Background(), "status", "status", "123", body)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse status json")
	})
}

func TestParseEvent_EmptyBodyFailsOnTypedPayload(t *testing.T) {
	// fillCommon accepts an empty body, but type-specific fillers still require JSON.
	_, err := ParseEvent(context.Background(), "push", "push", "123", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse push/ref json")
}

func TestParseEvent_ReviewApprovedAndRejected(t *testing.T) {
	approvedBody := mustJSON(t, map[string]any{
		"repository": map[string]any{"fullName": "org/app"},
		"sender":     map[string]any{"name": "approver"},
		"number":     3,
		"pullRequest": map[string]any{
			"title": "Ready",
			"state": "open",
		},
	})
	approved, err := ParseEvent(context.Background(), "pull_request_approved", "pull_request_review_approved", "123", approvedBody)
	require.NoError(t, err)
	assert.Equal(t, events.PullRequestReviewApproved, approved.Type)

	rejectedBody := mustJSON(t, map[string]any{
		"repository": map[string]any{"fullName": "org/app"},
		"sender":     map[string]any{"name": "reviewer"},
		"number":     4,
		"pullRequest": map[string]any{
			"title": "Needs work",
			"state": "open",
		},
	})
	rejected, err := ParseEvent(context.Background(), "pull_request_rejected", "pull_request_review_rejected", "123", rejectedBody)
	require.NoError(t, err)
	assert.Equal(t, events.PullRequestReviewRejected, rejected.Type)
}
