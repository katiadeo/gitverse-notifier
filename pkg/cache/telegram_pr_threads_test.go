package cache

import (
	"os"
	"path/filepath"
	"testing"

	"gitverse-notifier/pkg/config"
	"gitverse-notifier/pkg/events"

	"github.com/stretchr/testify/require"
)

func TestTelegramPRThreads_PersistsCard(t *testing.T) {
	root := t.TempDir()
	config.GlobalConfig = &config.Config{Root: root}

	first := NewTelegramPRThreads()
	first.Put(&TelegramPRCard{
		RootID:     99,
		CommentID:  100,
		Repository: "org/app",
		Number:     412,
		Title:      "APP-1183 [FE] form",
		Comments:   []events.CommentInfo{{Author: events.Actor{TgTag: "alice"}, Body: "looks good"}},
	})

	second := NewTelegramPRThreads()
	byNumber := second.Get("org/app", "", 412)
	require.NotNil(t, byNumber)
	require.Equal(t, 100, byNumber.ResolvedMessageID())
	require.Zero(t, byNumber.RootID)
	require.Zero(t, byNumber.CommentID)

	byTitle := second.Get("org/app", "Draft: APP-1183 [FE] form", 0)
	require.NotNil(t, byTitle)
	require.Equal(t, "alice", byTitle.Comments[0].Author.TgTag)
	require.Equal(t, "looks good", byTitle.Comments[0].Body)

	byIssue := second.Get("org/app", "APP-1183 totally different subtitle", 0)
	require.NotNil(t, byIssue)
	require.Equal(t, 100, byIssue.ResolvedMessageID())

	second.Remove("org/app", "APP-1183 [FE] form", 412)
	require.Nil(t, second.Get("org/app", "", 412))
}

func TestCommentLockKeyPrefersPRNumber(t *testing.T) {
	require.Equal(t, "org/app|#412", CommentLockKey("org/app", "APP-1183 [FE] form", 412))
	require.Equal(t, "org/app|key:APP-1183", CommentLockKey("org/app", "Draft: APP-1183 other", 0))
	require.NotEqual(t,
		CommentLockKey("org/app", "APP-1183 [FE] form", 412),
		CommentLockKey("org/app", "Draft: APP-1183 other", 0),
	)
}

func TestTelegramPRThreads_FindsCardByAnyIssueKey(t *testing.T) {
	config.GlobalConfig = &config.Config{}
	s := NewTelegramPRThreads()
	s.Put(&TelegramPRCard{
		RootID:     10,
		Repository: "org/app",
		Number:     214,
		Title:      "APP-1168, APP-1167 [FE] original",
		URL:        "https://git.example/org/app/pulls/214",
	})

	got := s.Get("org/app", "APP-1167 , APP-1168 [FE] later comment", 0)
	require.NotNil(t, got)
	require.Equal(t, 214, got.Number)
	require.Equal(t, 10, got.ResolvedMessageID())
	require.Equal(t, "https://git.example/org/app/pulls/214", got.URL)
}

func TestTelegramPRThreads_LoadReindexesAllIssueKeys(t *testing.T) {
	root := t.TempDir()
	config.GlobalConfig = &config.Config{Root: root}
	path := filepath.Join(root, "data", telegramPRThreadsFileName)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(`{"cards":{"org/app|key:APP-1168":{"root_id":10,"repository":"org/app","number":214,"title":"APP-1168, APP-1167 [FE] original","url":"https://git.example/org/app/pulls/214"}}}`), 0o644))

	loaded := NewTelegramPRThreads()
	got := loaded.Get("org/app", "APP-1167 later", 0)
	require.NotNil(t, got)
	require.Equal(t, 214, got.Number)
	require.Equal(t, 10, got.ResolvedMessageID())
}

func TestTelegramPRThreads_MergesOpenedAndComment(t *testing.T) {
	config.GlobalConfig = &config.Config{}
	s := NewTelegramPRThreads()
	s.Put(&TelegramPRCard{
		RootID:     10,
		Opened:     true,
		Repository: "org/app",
		Number:     412,
		Title:      "APP-1183 [FE] form",
	})
	s.Put(&TelegramPRCard{
		CommentID:  20,
		Rich:       true,
		Repository: "org/app",
		Title:      "APP-1183 other",
		Comments:   []events.CommentInfo{{Author: events.Actor{TgTag: "bob"}, Body: "please check"}},
	})

	got := s.Get("org/app", "", 412)
	require.NotNil(t, got)
	require.Equal(t, 20, got.ResolvedMessageID())
	require.True(t, got.Opened)
	require.Equal(t, "bob", got.Comments[0].Author.TgTag)
	require.Equal(t, "please check", got.Comments[0].Body)
}

func TestTelegramPRThreads_MigratesLegacyAuthors(t *testing.T) {
	config.GlobalConfig = &config.Config{}
	s := NewTelegramPRThreads()
	s.Put(&TelegramPRCard{
		RootID:     10,
		Repository: "org/app",
		Number:     412,
		Title:      "APP-1183 [FE] form",
		Authors:    []events.Actor{{TgTag: "alice"}},
	})

	got := s.Get("org/app", "", 412)
	require.NotNil(t, got)
	require.Len(t, got.Comments, 1)
	require.Equal(t, "alice", got.Comments[0].Author.TgTag)
	require.Nil(t, got.Authors)
}

func TestTelegramPRThreads_ClearsReviewersOnPut(t *testing.T) {
	config.GlobalConfig = &config.Config{}
	s := NewTelegramPRThreads()
	s.Put(&TelegramPRCard{
		Repository: "org/app",
		Number:     412,
		Reviewers:  []events.Actor{{Login: "alice"}, {Login: "bob"}},
	})
	s.Put(&TelegramPRCard{
		Repository: "org/app",
		Number:     412,
		Reviewers:  nil,
	})

	got := s.Get("org/app", "", 412)
	require.NotNil(t, got)
	require.Empty(t, got.Reviewers)
}

func TestTelegramPRThreads_ClearsCommentsOnPut(t *testing.T) {
	config.GlobalConfig = &config.Config{}
	s := NewTelegramPRThreads()
	s.Put(&TelegramPRCard{
		MessageID:  42,
		Repository: "org/app",
		Number:     412,
		Title:      "APP-1183 [FE] form",
		Comments: []events.CommentInfo{
			{ID: 1, Author: events.Actor{TgTag: "alice"}, Body: "only"},
		},
	})
	s.Put(&TelegramPRCard{
		MessageID:  42,
		Repository: "org/app",
		Number:     412,
		Title:      "APP-1183 [FE] form",
		Comments:   nil,
	})

	got := s.Get("org/app", "", 412)
	require.NotNil(t, got)
	require.Equal(t, 42, got.ResolvedMessageID())
	require.Empty(t, got.Comments)
}

func TestTelegramPRThreads_AmbiguousIssueKeyPrefersExactTitleOverReindexed(t *testing.T) {
	config.GlobalConfig = &config.Config{}
	s := NewTelegramPRThreads()
	s.Put(&TelegramPRCard{
		MessageID:  10,
		Repository: "org/app",
		Number:     214,
		Title:      "APP-1659 original",
		Closed:     true,
	})
	s.Put(&TelegramPRCard{
		MessageID:  99,
		Repository: "org/app",
		Number:     530,
		Title:      "APP-1659 follow-up",
		Opened:     true,
		Comments:   []events.CommentInfo{{Body: "on 530"}},
	})

	got := s.Get("org/app", "APP-1659 original", 0)
	require.NotNil(t, got)
	require.Equal(t, 214, got.Number)
	require.Equal(t, "APP-1659 original", got.Title)

	still530 := s.Get("org/app", "", 530)
	require.NotNil(t, still530)
	require.Equal(t, "APP-1659 follow-up", still530.Title)
	require.Equal(t, "on 530", still530.Comments[0].Body)
}

func TestTelegramPRThreads_GetMissing(t *testing.T) {
	config.GlobalConfig = &config.Config{}
	s := NewTelegramPRThreads()
	require.Nil(t, s.Get("org/app", "missing", 1))
}

func TestTelegramPRThreads_IgnoresIssueKeyWhenPRNumberDiffers(t *testing.T) {
	config.GlobalConfig = &config.Config{}
	s := NewTelegramPRThreads()
	s.Put(&TelegramPRCard{
		MessageID:  10,
		Repository: "org/app",
		Number:     214,
		Title:      "APP-1659 form work",
		Closed:     true,
	})

	got := s.Get("org/app", "APP-1659 another MR", 530)
	require.Nil(t, got)

	still := s.Get("org/app", "", 214)
	require.NotNil(t, still)
	require.True(t, still.Closed)
	require.Equal(t, 10, still.ResolvedMessageID())
}

func TestTelegramPRThreads_NewPRReindexesIssueKeyWithoutMergingOld(t *testing.T) {
	config.GlobalConfig = &config.Config{}
	s := NewTelegramPRThreads()
	s.Put(&TelegramPRCard{
		MessageID:  10,
		Repository: "org/app",
		Number:     214,
		Title:      "APP-1659 original",
		Closed:     true,
	})
	s.Put(&TelegramPRCard{
		MessageID:  99,
		Repository: "org/app",
		Number:     530,
		Title:      "APP-1659 follow-up",
		Opened:     true,
	})

	byIssue := s.Get("org/app", "APP-1659 later", 0)
	require.NotNil(t, byIssue)
	require.Equal(t, 530, byIssue.Number)
	require.Equal(t, 99, byIssue.ResolvedMessageID())
	require.False(t, byIssue.Closed)

	old := s.Get("org/app", "", 214)
	require.NotNil(t, old)
	require.True(t, old.Closed)
	require.Equal(t, 10, old.ResolvedMessageID())
}

func TestTelegramPRThreads_LoadsLegacyIDs(t *testing.T) {
	root := t.TempDir()
	config.GlobalConfig = &config.Config{Root: root}
	s := NewTelegramPRThreads()
	require.NoError(t, os.MkdirAll(filepath.Dir(s.path), 0o755))
	require.NoError(t, os.WriteFile(s.path, []byte(`{"ids":{"org/app|#412":99}}`), 0o644))

	loaded := NewTelegramPRThreads()
	got := loaded.Get("org/app", "", 412)
	require.NotNil(t, got)
	require.Equal(t, 99, got.ResolvedMessageID())
}

func TestTelegramPRThreads_RenameDropsOldTitleKey(t *testing.T) {
	config.GlobalConfig = &config.Config{}
	s := NewTelegramPRThreads()
	s.Put(&TelegramPRCard{
		MessageID:  10,
		Repository: "org/app",
		Number:     214,
		Title:      "plain original title",
		Comments:   []events.CommentInfo{{ID: 1, Body: "c1"}},
	})
	s.Put(&TelegramPRCard{
		MessageID:  10,
		Repository: "org/app",
		Number:     214,
		Title:      "plain renamed title",
		Comments:   []events.CommentInfo{{ID: 1, Body: "c1"}, {ID: 2, Body: "c2"}},
	})

	require.Nil(t, s.Get("org/app", "plain original title", 0))
	got := s.Get("org/app", "plain renamed title", 0)
	require.NotNil(t, got)
	require.Len(t, got.Comments, 2)
	require.Equal(t, 10, got.ResolvedMessageID())
}

func TestTelegramPRThreads_LoadPrefersNumberKeyOverStaleTitle(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o755))
	file := filepath.Join(dataDir, telegramPRThreadsFileName)
	content := `{
  "cards": {
    "org/app|#214": {
      "message_id": 10,
      "repository": "org/app",
      "number": 214,
      "title": "plain renamed title",
      "comments": [{"id": 1, "body": "kept"}]
    },
    "org/app|plain original title": {
      "message_id": 10,
      "repository": "org/app",
      "number": 214,
      "title": "plain original title",
      "comments": [{"id": 1, "body": "kept"}, {"id": 2, "body": "deleted-but-stale"}]
    }
  }
}`
	require.NoError(t, os.WriteFile(file, []byte(content), 0o644))

	config.GlobalConfig = &config.Config{Root: root}
	s := NewTelegramPRThreads()

	byNum := s.Get("org/app", "", 214)
	require.NotNil(t, byNum)
	require.Len(t, byNum.Comments, 1)
	require.Equal(t, "kept", byNum.Comments[0].Body)
	require.Nil(t, s.Get("org/app", "plain original title", 0))
}
