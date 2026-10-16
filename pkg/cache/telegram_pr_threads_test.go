package cache

import (
	"testing"

	"gitverse-notifier/pkg/config"

	"github.com/stretchr/testify/require"
)

func TestTelegramPRThreads_PersistsAndFindsByNumberOrTitle(t *testing.T) {
	root := t.TempDir()
	config.GlobalConfig = &config.Config{Root: root}

	first := NewTelegramPRThreads()
	first.Put("org/app", "APP-1183 [FE] form", 412, 99)

	second := NewTelegramPRThreads()
	require.Equal(t, 99, second.Get("org/app", "", 412))
	require.Equal(t, 99, second.Get("org/app", "Draft: APP-1183 [FE] form", 0))
}

func TestTelegramPRThreads_GetMissing(t *testing.T) {
	config.GlobalConfig = &config.Config{}
	s := NewTelegramPRThreads()
	require.Equal(t, 0, s.Get("org/app", "missing", 1))
}
