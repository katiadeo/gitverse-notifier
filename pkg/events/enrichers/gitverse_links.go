package enrichers

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"gitverse-notifier/pkg/config"
	"gitverse-notifier/pkg/events"
	"gitverse-notifier/pkg/events/parsers"
	"gitverse-notifier/pkg/settings"
)

type GitverseLinks struct {
	manager *settings.SettingsManager
}

func NewGitverseLinks(manager *settings.SettingsManager) *GitverseLinks {
	return &GitverseLinks{manager: manager}
}

func (e *GitverseLinks) Name() string {
	return "enricher.gitverse-links"
}

func (e *GitverseLinks) Skip(event *events.Event) bool {
	return event == nil
}

func (e *GitverseLinks) Enrich(_ context.Context, event *events.Event) error {
	baseURL := config.GlobalConfig.GitverseBaseURL

	e.addUserURL(baseURL, &event.Sender)
	e.addUserURL(baseURL, &event.PullRequest.Author)
	e.addUserURL(baseURL, &event.Comment.Author)

	if event.PullRequest.Number <= 0 {
		event.PullRequest.Number = parsers.PRNumberFromURL(event.PullRequest.URL)
	}
	if event.Repository == "" {
		event.Repository = parsers.RepoPathFromURL(event.PullRequest.URL)
	}
	if event.Repository == "" {
		return nil
	}

	repoPath := strings.Trim(event.Repository, "/")
	event.RepositoryURL = fmt.Sprintf("%s/%s", baseURL, repoPath)

	if event.Branch != "" {
		event.BranchURL = repoBranchURL(baseURL, repoPath, event.Branch)
	}
	if event.PullRequest.HeadRef != "" {
		event.PullRequest.HeadURL = repoBranchURL(baseURL, repoPath, event.PullRequest.HeadRef)
	}
	if event.PullRequest.BaseRef != "" {
		event.PullRequest.BaseURL = repoBranchURL(baseURL, repoPath, event.PullRequest.BaseRef)
	}

	if event.PullRequest.Number > 0 && baseURL != "" {
		event.PullRequest.URL = fmt.Sprintf("%s/%s/pulls/%d", strings.TrimRight(baseURL, "/"), repoPath, event.PullRequest.Number)
	}

	return nil
}

func (e *GitverseLinks) addUserURL(baseURL string, actor *events.Actor) {
	if strings.TrimSpace(actor.URL) != "" || baseURL == "" {
		return
	}
	name := strings.TrimSpace(actor.Name)
	if name != "" {
		actor.URL = fmt.Sprintf("%s/%s", baseURL, url.PathEscape(name))
	}
}

func repoBranchURL(baseURL, repoPath, ref string) string {
	if baseURL == "" || repoPath == "" || ref == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s/src/branch/%s", baseURL, repoPath, url.PathEscape(ref))
}
