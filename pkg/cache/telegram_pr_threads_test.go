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
		Authors:    []events.Actor{{TgTag: "alice"}},
	})

	second := NewTelegramPRThreads()
	byNumber := second.Get("org/app", "", 412)
	require.NotNil(t, byNumber)
	require.Equal(t, 99, byNumber.RootID)
	require.Equal(t, 100, byNumber.CommentID)

	byTitle := second.Get("org/app", "Draft: APP-1183 [FE] form", 0)
	require.NotNil(t, byTitle)
	require.Equal(t, "alice", byTitle.Authors[0].TgTag)

	second.Remove("org/app", "APP-1183 [FE] form", 412)
	require.Nil(t, second.Get("org/app", "", 412))
}

func TestTelegramPRThreads_GetMissing(t *testing.T) {
	config.GlobalConfig = &config.Config{}
	s := NewTelegramPRThreads()
	require.Nil(t, s.Get("org/app", "missing", 1))
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
	require.Equal(t, 99, got.RootID)
}
