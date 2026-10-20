package parsers

import (
	"encoding/json"
	"fmt"
	"strings"

	"gitverse-notifier/pkg/events"
)

type prBranchPayload struct {
	Ref   string `json:"ref"`
	Label string `json:"label"`
}

type pullRequestBodyPayload struct {
	Number                  flexInt         `json:"number"`
	Index                   flexInt         `json:"index"`
	Title                   string          `json:"title"`
	Body                    string          `json:"body"`
	State                   string          `json:"state"`
	Merged                  bool            `json:"merged"`
	MergedAt                string          `json:"merged_at"`
	MergedAtCamel           string          `json:"mergedAt"`
	MergeCommitSHA          string          `json:"merge_commit_sha"`
	MergeCommitSHACamel     string          `json:"mergeCommitSha"`
	MergedBy                actorPayload    `json:"merged_by"`
	MergedByCamel           actorPayload    `json:"mergedBy"`
	HTMLURL                 string          `json:"html_url"`
	HTMLUrl                 string          `json:"htmlUrl"`
	URL                     string          `json:"url"`
	User                    actorPayload    `json:"user"`
	Head                    prBranchPayload `json:"head"`
	Base                    prBranchPayload `json:"base"`
	RequestedReviewers      []actorPayload  `json:"requested_reviewers"`
	RequestedReviewersCamel []actorPayload  `json:"requestedReviewers"`
	Reviewers               []actorPayload  `json:"reviewers"`
}

func (p pullRequestBodyPayload) isMerged() bool {
	if p.Merged {
		return true
	}
	if strings.EqualFold(strings.TrimSpace(p.State), "merged") {
		return true
	}
	if firstNonEmpty(p.MergedAt, p.MergedAtCamel) != "" {
		return true
	}
	if firstNonEmpty(p.MergeCommitSHA, p.MergeCommitSHACamel) != "" {
		return true
	}
	return p.mergedBy().toActor() != (events.Actor{})
}

func (p pullRequestBodyPayload) mergedBy() actorPayload {
	if p.MergedBy.toActor() != (events.Actor{}) {
		return p.MergedBy
	}
	return p.MergedByCamel
}

func (p pullRequestBodyPayload) url() string {
	return firstNonEmpty(p.HTMLURL, p.HTMLUrl, toPullHTMLURL(p.URL))
}

func (p pullRequestBodyPayload) number() int {
	return firstPositive(p.Number.Int(), p.Index.Int())
}

type pullRequestPayload struct {
	Number                  flexInt                `json:"number"`
	Index                   flexInt                `json:"index"`
	PullRequest             pullRequestBodyPayload `json:"pullRequest"`
	PullRequestSnake        pullRequestBodyPayload `json:"pull_request"`
	RequestedReviewer       actorPayload           `json:"requested_reviewer"`
	RequestedReviewerCamel  actorPayload           `json:"requestedReviewer"`
	RequestedReviewers      []actorPayload         `json:"requested_reviewers"`
	RequestedReviewersCamel []actorPayload         `json:"requestedReviewers"`
	Review                  struct {
		Type    string `json:"type"`
		Content string `json:"content"`
	} `json:"review"`
	Sender actorPayload `json:"sender"`
}

func (p pullRequestPayload) pr() pullRequestBodyPayload {
	return mergePRBody(p.PullRequest, p.PullRequestSnake)
}

func mergePRBody(parts ...pullRequestBodyPayload) pullRequestBodyPayload {
	var out pullRequestBodyPayload
	for _, p := range parts {
		if out.number() <= 0 && p.number() > 0 {
			out.Number = p.Number
			out.Index = p.Index
		}
		if out.url() == "" {
			if u := p.url(); u != "" {
				out.HTMLURL = u
			}
		}
		if out.Title == "" {
			out.Title = p.Title
		}
		if out.Body == "" {
			out.Body = p.Body
		}
		if out.State == "" {
			out.State = p.State
		}
		if !out.Merged {
			out.Merged = p.isMerged()
		}
		if out.MergedAt == "" {
			out.MergedAt = firstNonEmpty(p.MergedAt, p.MergedAtCamel)
		}
		if out.MergeCommitSHA == "" {
			out.MergeCommitSHA = firstNonEmpty(p.MergeCommitSHA, p.MergeCommitSHACamel)
		}
		if out.mergedBy().toActor() == (events.Actor{}) {
			if by := p.mergedBy(); by.toActor() != (events.Actor{}) {
				out.MergedBy = by
			}
		}
		if out.User.toActor() == (events.Actor{}) {
			out.User = p.User
		}
		if firstNonEmpty(out.Head.Ref, out.Head.Label) == "" {
			out.Head = p.Head
		}
		if firstNonEmpty(out.Base.Ref, out.Base.Label) == "" {
			out.Base = p.Base
		}
		if len(out.RequestedReviewers) == 0 {
			switch {
			case len(p.RequestedReviewers) > 0:
				out.RequestedReviewers = p.RequestedReviewers
			case len(p.RequestedReviewersCamel) > 0:
				out.RequestedReviewers = p.RequestedReviewersCamel
			case len(p.Reviewers) > 0:
				out.RequestedReviewers = p.Reviewers
			}
		}
	}
	return out
}

func fillPullRequest(event *events.Event, body []byte) error {
	var payload pullRequestPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("failed to parse pull request json: %w", err)
	}

	pr := payload.pr()
	url := pr.url()
	event.PullRequest = events.PullRequestInfo{
		Number: firstPositive(
			pr.number(),
			payload.Number.Int(),
			payload.Index.Int(),
			prNumberFromURL(url),
		),
		Title:          pr.Title,
		Body:           pr.Body,
		State:          pr.State,
		Merged:         pr.isMerged(),
		URL:            url,
		Author:         pr.User.toActor(),
		HeadRef:        firstNonEmpty(pr.Head.Ref, pr.Head.Label),
		BaseRef:        firstNonEmpty(pr.Base.Ref, pr.Base.Label),
		Reviewers:      requestedReviewers(event.Type, payload, pr),
		ReviewersExact: reviewersExact(payload, pr),
	}

	if by := pr.mergedBy().toActor(); by != (events.Actor{}) {
		event.MergedBy = by
	}

	if event.PullRequest.Author == (events.Actor{}) {
		event.PullRequest.Author = event.Sender
	}

	if payload.Review.Content != "" || event.Type == events.PullRequestReviewComment {
		event.Comment = events.CommentInfo{
			Body:   payload.Review.Content,
			Author: event.Sender,
		}
	}

	return nil
}
