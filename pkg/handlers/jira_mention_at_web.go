package handlers

import (
	"context"
	"errors"
	"strings"

	"gitverse-notifier/pkg/events"
	"gitverse-notifier/pkg/integrations/jira"
	"gitverse-notifier/pkg/logger"
	"gitverse-notifier/pkg/settings"
)

type JiraMentionAtWeb struct {
	client jiraIssueClient
}

func NewJiraMentionAtWeb(client *jira.JiraClient) *JiraMentionAtWeb {
	var api jiraIssueClient
	if client != nil {
		api = client
	}
	return &JiraMentionAtWeb{client: api}
}

func (a *JiraMentionAtWeb) Name() string {
	return "jira.mention_at_web"
}

func (a *JiraMentionAtWeb) Ready() bool {
	return a.client != nil
}

func (a *JiraMentionAtWeb) Run(ctx context.Context, event *events.Event, rule *settings.ActionRule) error {
	if event.PullRequest.URL == "" {
		logger.Info(ctx, "no pull request url found, skipping action",
			"action", a.Name(),
			"repository", event.Repository,
			"title", event.PullRequest.Title,
		)
		return nil
	}

	if len(event.IssueKeys) == 0 {
		logger.Info(ctx, "no issue keys found, skipping action",
			"action", a.Name(),
			"repository", event.Repository,
			"title", event.PullRequest.Title,
			"branch", event.Branch,
		)
		return nil
	}

	logger.Info(ctx, "adding jira web mentions",
		"action", a.Name(),
		"event", event.Type,
		"issue-keys", len(event.IssueKeys),
		"keys", strings.Join(event.IssueKeys, ","),
	)

	var errs []error
	for _, key := range event.IssueKeys {
		if _, err := a.client.AddMentionedAtWeb(key, event.PullRequest.URL, event.PullRequest.Title); err != nil {
			logger.Error(ctx, "failed to add mentioned-at web link to issue",
				"action", a.Name(),
				"event", event.Type,
				"issue", key,
				"url", event.PullRequest.URL,
				"err", err,
			)
			errs = append(errs, err)
			continue
		}
		logger.Info(ctx, "successfully added mentioned-at web link to issue",
			"action", a.Name(),
			"event", event.Type,
			"issue", key,
			"url", event.PullRequest.URL,
			"title", event.PullRequest.Title,
		)
	}

	return errors.Join(errs...)
}
