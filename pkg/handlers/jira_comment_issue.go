package handlers

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gitverse-notifier/pkg/events"
	"gitverse-notifier/pkg/integrations/jira"
	"gitverse-notifier/pkg/logger"
	"gitverse-notifier/pkg/settings"
	"gitverse-notifier/pkg/templates"

	gojira "github.com/andygrunwald/go-jira"
)

const defaultJiraTemplate = "jira/default"

type jiraIssueClient interface {
	AddComment(taskCode, body string) (*gojira.Comment, error)
	AddMentionedAtWeb(taskCode, url, title string) (*gojira.RemoteLink, error)
}

type JiraCommentIssue struct {
	client    jiraIssueClient
	templates *templates.Engine
}

func NewJiraCommentIssue(client *jira.JiraClient, engine *templates.Engine) *JiraCommentIssue {
	var api jiraIssueClient
	if client != nil {
		api = client
	}
	return &JiraCommentIssue{client: api, templates: engine}
}

func (a *JiraCommentIssue) Name() string {
	return "jira.comment_issue"
}

func (a *JiraCommentIssue) Ready() bool {
	return a.client != nil
}

func (a *JiraCommentIssue) Run(ctx context.Context, event *events.Event, rule *settings.ActionRule) error {
	templateName := strings.TrimSpace(rule.Template)
	if templateName == "" {
		templateName = defaultJiraTemplate
	}

	body, err := a.templates.Render(templateName, templates.DataFromEvent(event))
	if err != nil {
		return fmt.Errorf("failed to render template %s: %w", templateName, err)
	}
	logger.Debug(ctx, "rendered issue comment body", "body", body)

	if len(event.IssueKeys) == 0 {
		logger.Info(ctx, "no issue keys found, skipping action",
			"action", a.Name(),
			"repository", event.Repository,
			"title", event.PullRequest.Title,
			"branch", event.Branch,
		)
		return nil
	}

	logger.Info(ctx, "commenting on jira issues",
		"action", a.Name(),
		"event", event.Type,
		"issue-keys", len(event.IssueKeys),
		"keys", strings.Join(event.IssueKeys, ","),
	)

	var errs []error
	for _, key := range event.IssueKeys {
		if _, err := a.client.AddComment(key, body); err != nil {
			logger.Error(ctx, "failed to comment on issue",
				"action", a.Name(),
				"event", event.Type,
				"issue", key,
				"err", err,
			)
			errs = append(errs, err)
			continue
		}

		logger.Info(ctx, "successfully commented on issue",
			"action", a.Name(),
			"event", event.Type,
			"issue", key,
		)
	}
	return errors.Join(errs...)
}
