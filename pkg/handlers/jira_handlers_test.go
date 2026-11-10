package handlers

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"gitverse-notifier/pkg/config"
	"gitverse-notifier/pkg/events"
	"gitverse-notifier/pkg/settings"
	"gitverse-notifier/pkg/templates"

	gojira "github.com/andygrunwald/go-jira"
	"github.com/stretchr/testify/require"
)

type fakeJiraClient struct {
	commentCalls []string
	mentionCalls []string
	failOn       map[string]error
}

func (f *fakeJiraClient) AddComment(taskCode, body string) (*gojira.Comment, error) {
	f.commentCalls = append(f.commentCalls, taskCode)
	if err, ok := f.failOn[taskCode]; ok {
		return nil, err
	}
	return &gojira.Comment{Body: body}, nil
}

func (f *fakeJiraClient) AddMentionedAtWeb(taskCode, url, title string) (*gojira.RemoteLink, error) {
	f.mentionCalls = append(f.mentionCalls, taskCode)
	if err, ok := f.failOn[taskCode]; ok {
		return nil, err
	}
	return &gojira.RemoteLink{GlobalID: url}, nil
}

func TestJiraCommentIssue_ContinuesAfterPartialFailure(t *testing.T) {
	engine := testTemplateEngine(t)
	fake := &fakeJiraClient{
		failOn: map[string]error{
			"APP-1339": errors.New("boom first"),
		},
	}
	handler := &JiraCommentIssue{client: fake, templates: engine}

	event := &events.Event{
		Type:       events.PullRequestOpened,
		Repository: "bks-frontend/fcs-frontend",
		Sender:     events.Actor{Name: "tmukhortova", URL: "https://git.example/tmukhortova"},
		PullRequest: events.PullRequestInfo{
			Number: 280,
			Title:  `APP-1339, APP-1204 Добавлен список`,
			URL:    "https://git.example/bks-frontend/fcs-frontend/pulls/280",
		},
		IssueKeys: []string{"APP-1339", "APP-1204"},
	}

	err := handler.Run(context.Background(), event, &settings.ActionRule{
		Template: "jira/pr_opened",
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "boom first")
	require.Equal(t, []string{"APP-1339", "APP-1204"}, fake.commentCalls)
}

func TestJiraMentionAtWeb_ContinuesAfterPartialFailure(t *testing.T) {
	fake := &fakeJiraClient{
		failOn: map[string]error{
			"APP-1339": errors.New("boom first"),
		},
	}
	handler := &JiraMentionAtWeb{client: fake}

	event := &events.Event{
		Type:       events.PullRequestOpened,
		Repository: "bks-frontend/fcs-frontend",
		PullRequest: events.PullRequestInfo{
			Title: "APP-1339, APP-1204 Добавлен список",
			URL:   "https://git.example/bks-frontend/fcs-frontend/pulls/280",
		},
		IssueKeys: []string{"APP-1339", "APP-1204"},
	}

	err := handler.Run(context.Background(), event, &settings.ActionRule{})
	require.Error(t, err)
	require.ErrorContains(t, err, "boom first")
	require.Equal(t, []string{"APP-1339", "APP-1204"}, fake.mentionCalls)
}

func TestJiraCommentIssue_AllKeysSucceed(t *testing.T) {
	engine := testTemplateEngine(t)
	fake := &fakeJiraClient{}
	handler := &JiraCommentIssue{client: fake, templates: engine}

	event := &events.Event{
		Type:       events.PullRequestOpened,
		Repository: "bks-frontend/fcs-frontend",
		Sender:     events.Actor{Name: "alice", URL: "https://git.example/alice"},
		PullRequest: events.PullRequestInfo{
			Number: 1,
			Title:  "APP-1/APP-2: form",
			URL:    "https://git.example/org/app/pulls/1",
		},
		IssueKeys: []string{"APP-1", "APP-2"},
	}

	err := handler.Run(context.Background(), event, &settings.ActionRule{Template: "jira/pr_opened"})
	require.NoError(t, err)
	require.Equal(t, []string{"APP-1", "APP-2"}, fake.commentCalls)
}

func testTemplateEngine(t *testing.T) *templates.Engine {
	t.Helper()

	root := findModuleRoot(t)
	prev := config.GlobalConfig
	config.GlobalConfig = &config.Config{
		TemplatesDirPath:   filepath.Join(root, "templates"),
		TemplatesPattern:   "**/*.tmpl",
		JiraURL:            "https://jira.example.com",
		TelegramParseMode:  "MarkdownV2",
	}
	t.Cleanup(func() { config.GlobalConfig = prev })

	engine, err := templates.SetupTemplateEngine()
	require.NoError(t, err)
	return engine
}

func findModuleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
