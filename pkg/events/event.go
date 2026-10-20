package events

import (
	"fmt"
	"regexp"
	"strings"
)

var pullNumberFromURLRe = regexp.MustCompile(`(?i)/(?:pulls?|issues)/(\d+)`)

type EventType string

const (
	Unknown EventType = "unknown"

	PullRequestOpened               EventType = "pull_request.opened"
	PullRequestClosed               EventType = "pull_request.closed"
	PullRequestEdited               EventType = "pull_request.edited"
	PullRequestSynchronized         EventType = "pull_request.synchronized"
	PullRequestReviewRequested      EventType = "pull_request.review_requested"
	PullRequestReviewRequestRemoved EventType = "pull_request.review_request_removed"
	PullRequestReviewApproved       EventType = "pull_request.review_approved"
	PullRequestReviewRejected       EventType = "pull_request.review_rejected"
	PullRequestReviewComment        EventType = "pull_request.review_comment"
	PullRequestComment              EventType = "pull_request.comment"

	BranchPush    EventType = "branch.push"
	BranchCreated EventType = "branch.created"
	BranchDeleted EventType = "branch.deleted"

	CICDStatus EventType = "cicd.status"
)

type Actor struct {
	ID    int64
	Name  string
	Login string
	Email string
	URL   string
	TgTag string
}

type PullRequestInfo struct {
	Number         int
	Title          string
	Body           string
	State          string
	Merged         bool
	Author         Actor
	URL            string
	HeadRef        string
	HeadURL        string
	BaseRef        string
	BaseURL        string
	Reviewers      []Actor
	ReviewersExact bool
}

type CommentInfo struct {
	ID int `json:"id,omitempty"`
	// CreatedAt is the immutable creation timestamp supplied by GitVerse.  Some
	// issue_comment webhooks omit both the comment ID and URL, so it is the only
	// stable way to match a later edit or deletion to its original comment.
	CreatedAt string `json:"created_at,omitempty"`
	Body      string `json:"body,omitempty"`
	// PreviousBody is present only on an edit webhook. It lets us migrate cards
	// created before GitVerse started sending a usable comment identity.
	PreviousBody string  `json:"-"`
	Author       Actor   `json:"author"`
	URL          string  `json:"url,omitempty"`
	Mentions     []Actor `json:"mentions,omitempty"`
	Edited       bool    `json:"edited,omitempty"`
	ReplyTo      int     `json:"reply_to,omitempty"`
	ThreadID     int     `json:"thread_id,omitempty"`
}

type PushInfo struct {
	Before       string
	After        string
	TotalCommits int
	CommitTitle  string
	URL          string
}

type StatusInfo struct {
	State       string
	Context     string
	Description string
	SHA         string
}

type Event struct {
	RequestId     string
	Type          EventType
	Action        string
	Ref           string
	Branch        string
	BranchURL     string
	Repository    string
	RepositoryURL string
	Sender        Actor
	PullRequest   PullRequestInfo
	Comment       CommentInfo
	Comments      []CommentInfo
	Approvals     []CommentInfo
	Rejections    []CommentInfo
	MergedBy      Actor
	Push          PushInfo
	Status        StatusInfo
	IssueKeys     []string
}

func ResolveEventType(event, eventType, action string) (EventType, error) {
	event = strings.ToLower(strings.TrimSpace(event))
	eventType = strings.ToLower(strings.TrimSpace(eventType))
	action = strings.ToLower(strings.TrimSpace(action))

	switch action {
	case "review_requested", "review_request":
		if isPullRequestEventFamily(event, eventType) {
			return PullRequestReviewRequested, nil
		}
	case "review_request_removed":
		if isPullRequestEventFamily(event, eventType) {
			return PullRequestReviewRequestRemoved, nil
		}
	}

	eventMapping := map[string]EventType{
		"push.push":     BranchPush,
		"create.create": BranchCreated,
		"delete.delete": BranchDeleted,
		"status.status": CICDStatus,

		"pull_request_approved.pull_request_review_approved": PullRequestReviewApproved,
		"pull_request_rejected.pull_request_review_rejected": PullRequestReviewRejected,
		"pull_request_comment.pull_request_review_comment":   PullRequestReviewComment,
		"issue_comment.pull_request_comment":                 PullRequestComment,
		"pull_request.pull_request_review_request":           PullRequestReviewRequested,
		"pull_request.pull_request_review_requested":         PullRequestReviewRequested,
		"pull_request.review_requested":                      PullRequestReviewRequested,
		"pull_request.review_request":                        PullRequestReviewRequested,
		"pull_request.pull_request_sync":                     PullRequestSynchronized,

		"pull_request.pull_request.opened":                 PullRequestOpened,
		"pull_request.pull_request.closed":                 PullRequestClosed,
		"pull_request.pull_request.edited":                 PullRequestEdited,
		"pull_request.pull_request.synchronized":           PullRequestSynchronized,
		"pull_request.pull_request.review_requested":       PullRequestReviewRequested,
		"pull_request.pull_request.review_request":         PullRequestReviewRequested,
		"pull_request.pull_request.review_request_removed": PullRequestReviewRequestRemoved,
		"pull_request.review_request_removed":              PullRequestReviewRequestRemoved,
		"pull_request.pull_request_review_request_removed": PullRequestReviewRequestRemoved,
	}

	if value, ok := eventMapping[fmt.Sprintf("%s.%s.%s", event, eventType, action)]; ok {
		return value, nil
	}

	if value, ok := eventMapping[fmt.Sprintf("%s.%s", event, eventType)]; ok {
		return value, nil
	}

	return Unknown, fmt.Errorf("failed to detect event type for %s.%s, action = %s", event, eventType, action)
}

func isPullRequestEventFamily(event, eventType string) bool {
	for _, value := range []string{event, eventType} {
		switch value {
		case "pull_request", "pull_request_review_request", "pull_request_review_requested",
			"pull_request_review_request_removed", "review_requested", "review_request",
			"review_request_removed":
			return true
		}
	}
	return false
}

func ParseEventType(s string) (EventType, bool) {
	s = strings.TrimSpace(s)
	switch EventType(s) {
	case PullRequestOpened, PullRequestClosed, PullRequestEdited, PullRequestSynchronized,
		PullRequestReviewRequested, PullRequestReviewRequestRemoved, PullRequestReviewApproved, PullRequestReviewRejected,
		PullRequestReviewComment, PullRequestComment,
		BranchPush, BranchCreated, BranchDeleted, CICDStatus:
		return EventType(s), true
	default:
		return Unknown, false
	}
}

func BranchName(ref string) string {
	ref = strings.TrimSpace(ref)
	for _, prefix := range []string{"refs/heads/", "refs/tags/"} {
		if strings.HasPrefix(ref, prefix) {
			return strings.TrimPrefix(ref, prefix)
		}
	}
	return ref
}

func (p PullRequestInfo) EffectiveState() string {
	if p.Merged || strings.EqualFold(strings.TrimSpace(p.State), "merged") {
		return "merged"
	}

	return p.State
}

func (p PullRequestInfo) EffectiveNumber() int {
	if p.Number > 0 {
		return p.Number
	}
	if match := pullNumberFromURLRe.FindStringSubmatch(p.URL); len(match) == 2 {
		var n int
		if _, err := fmt.Sscanf(match[1], "%d", &n); err == nil {
			return n
		}
	}
	return 0
}
