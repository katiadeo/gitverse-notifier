package templates

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"gitverse-notifier/pkg/config"
	"gitverse-notifier/pkg/events"
	"gitverse-notifier/pkg/logger"
)

type Data struct {
	Type          string
	Action        string
	Repository    string
	RepositoryURL string
	Ref           string
	Branch        string
	BranchURL     string
	Sender        events.Actor
	PullRequest   events.PullRequestInfo
	Comment       events.CommentInfo
	Comments      []events.CommentInfo
	Approvals     []events.CommentInfo
	Rejections    []events.CommentInfo
	MergedBy      events.Actor
	Push          events.PushInfo
	Status        events.StatusInfo
	IssueKeys     []string
}

func DataFromEvent(ev *events.Event) Data {
	return Data{
		Type:          string(ev.Type),
		Action:        ev.Action,
		Repository:    ev.Repository,
		RepositoryURL: ev.RepositoryURL,
		Ref:           ev.Ref,
		Branch:        ev.Branch,
		BranchURL:     ev.BranchURL,
		Sender:        ev.Sender,
		PullRequest:   ev.PullRequest,
		Comment:       ev.Comment,
		Comments:      commentInfosFromEvent(ev),
		Approvals:     ev.Approvals,
		Rejections:    ev.Rejections,
		MergedBy:      ev.MergedBy,
		Push:          ev.Push,
		Status:        ev.Status,
		IssueKeys:     ev.IssueKeys,
	}
}

type Engine struct {
	engine *template.Template
}

func funcMap() template.FuncMap {
	return template.FuncMap{
		"jiraEscape":           JiraEscape,
		"trim":                 strings.TrimSpace,
		"join":                 strings.Join,
		"jiraLink":             JiraLink,
		"mdLink":               MarkdownLink,
		"tgLink":               TelegramLink,
		"issueURL":             IssueURL,
		"mdIssueURLs":          MarkdownIssueURLs,
		"plain":                StripHTML,
		"tgPlain":              TelegramPlain,
		"tgPR":                 TelegramPR,
		"tgName":               TelegramName,
		"tgRichLink":           TelegramRichLink,
		"tgRichPR":             TelegramRichPR,
		"tgPRStateMarker":      TelegramPRStateMarker,
		"tgRichPlain":          TelegramRichPlain,
		"tgRichPRTitle":        TelegramRichPRTitle,
		"tgRichQuote":          TelegramRichQuote,
		"tgRichName":           TelegramRichName,
		"tgRichCopyableName":   TelegramRichCopyableName,
		"tgRichMention":        TelegramRichMention,
		"tgRichCommentSummary": TelegramRichCommentSummary,
		"reverseComments":      ReverseComments,
		"commentThreads":       CommentThreads,
	}
}

func commentInfosFromEvent(ev *events.Event) []events.CommentInfo {
	if ev == nil {
		return nil
	}
	if len(ev.Comments) > 0 {
		return ev.Comments
	}
	if ev.Comment.Author != (events.Actor{}) || strings.TrimSpace(ev.Comment.Body) != "" {
		return []events.CommentInfo{ev.Comment}
	}
	return nil
}

func SetupTemplateEngine() (*Engine, error) {
	cfg := config.GlobalConfig
	pattern := filepath.Join(cfg.TemplatesDirPath, cfg.TemplatesPattern)

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to glob templates by %s: %w", pattern, err)
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no templates found by %s", pattern)
	}

	root := template.New("").Funcs(funcMap())
	for _, path := range matches {
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read template %s: %w", path, err)
		}

		rel, err := filepath.Rel(cfg.TemplatesDirPath, path)
		name := strings.TrimSuffix(rel, filepath.Ext(rel))
		if _, err := root.New(name).Parse(string(content)); err != nil {
			return nil, fmt.Errorf("failed to parse template %s: %w", path, err)
		}

		logger.Debug(nil, "template registered", "action", "templates.setup", "template", name, "path", path)
	}

	return &Engine{engine: root}, nil
}

func (e *Engine) Render(name string, data Data) (string, error) {
	tmpl := e.engine.Lookup(name)
	if tmpl == nil {
		return "", fmt.Errorf("template %q not found", name)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template %q: %w", name, err)
	}

	return buf.String(), nil
}
