package enrichers

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"gitverse-notifier/pkg/config"
	"gitverse-notifier/pkg/events"
	"gitverse-notifier/pkg/settings"

	"github.com/stretchr/testify/require"
)

func TestJiraIssueKeys_MultiKeyTitles(t *testing.T) {
	manager := testJiraSettingsManager(t)

	tests := []struct {
		name  string
		title string
		want  []string
	}{
		{
			name:  "comma separated",
			title: "APP-1339, APP-1204 Добавлен список событий для представления День календаря",
			want:  []string{"APP-1339", "APP-1204"},
		},
		{
			name:  "slash separated",
			title: "APP-1482/APP-1483/APP-1484/APP-1485: Правка формы",
			want:  []string{"APP-1482", "APP-1483", "APP-1484", "APP-1485"},
		},
		{
			name:  "mixed separators",
			title: "APP-1 APP-2, APP-3 / APP-4 form",
			want:  []string{"APP-1", "APP-2", "APP-3", "APP-4"},
		},
		{
			name:  "single key",
			title: "APP-1742 [FE] Создание",
			want:  []string{"APP-1742"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &events.Event{
				Repository:  "org/app",
				PullRequest: events.PullRequestInfo{Title: tt.title},
			}
			err := NewJiraIssueKeys(manager).Enrich(context.Background(), event)
			require.NoError(t, err)
			require.Equal(t, tt.want, event.IssueKeys)
		})
	}
}

func TestJiraIssueKeys_DedupesAcrossTitleAndBranch(t *testing.T) {
	manager := testJiraSettingsManager(t)
	event := &events.Event{
		Repository:  "org/app",
		Branch:      "APP-1339",
		PullRequest: events.PullRequestInfo{Title: "APP-1339, APP-1204 form"},
	}
	err := NewJiraIssueKeys(manager).Enrich(context.Background(), event)
	require.NoError(t, err)
	require.Equal(t, []string{"APP-1339", "APP-1204"}, event.IssueKeys)
}

func testJiraSettingsManager(t *testing.T) *settings.SettingsManager {
	t.Helper()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "default.yml"),
		[]byte("repository: any\nallowed_jira_projects: []\n"),
		0o644,
	))

	prev := config.GlobalConfig
	config.GlobalConfig = &config.Config{ConfigsDirPath: dir}
	t.Cleanup(func() { config.GlobalConfig = prev })

	manager, err := settings.SetupSettingsManager()
	require.NoError(t, err)
	return manager
}
