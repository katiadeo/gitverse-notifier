package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gitverse-notifier/pkg/config"
	"gitverse-notifier/pkg/integrations/gitverse"
	"gitverse-notifier/pkg/logger"
)

const telegramPRThreadsFileName = "telegram-pr-threads.json"

type TelegramPRThreads struct {
	mu   sync.Mutex
	path string
	ids  map[string]int
}

func NewTelegramPRThreads() *TelegramPRThreads {
	s := &TelegramPRThreads{
		path: telegramPRThreadsPath(),
		ids:  map[string]int{},
	}
	s.load()
	return s
}

func (s *TelegramPRThreads) Get(repo, title string, number int) int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if number > 0 {
		if id := s.ids[telegramThreadNumberKey(repo, number)]; id > 0 {
			return id
		}
	}
	if strings.TrimSpace(title) != "" {
		return s.ids[telegramThreadTitleKey(repo, title)]
	}
	return 0
}

func (s *TelegramPRThreads) Put(repo, title string, number, messageID int) {
	if s == nil || messageID <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if number > 0 {
		s.ids[telegramThreadNumberKey(repo, number)] = messageID
	}
	if strings.TrimSpace(title) != "" {
		s.ids[telegramThreadTitleKey(repo, title)] = messageID
	}
	s.persistLocked()
}

func (s *TelegramPRThreads) persistLocked() {
	if s.path == "" {
		return
	}
	out := struct {
		IDs map[string]int `json:"ids"`
	}{IDs: s.ids}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		logger.Warn(nil, "failed to encode telegram PR threads", "err", err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		logger.Warn(nil, "failed to create telegram PR threads dir", "err", err, "path", s.path)
		return
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		logger.Warn(nil, "failed to write telegram PR threads", "err", err, "path", tmp)
		return
	}
	if err := os.Rename(tmp, s.path); err != nil {
		logger.Warn(nil, "failed to replace telegram PR threads", "err", err, "path", s.path)
		_ = os.Remove(tmp)
	}
}

func (s *TelegramPRThreads) load() {
	if s.path == "" {
		return
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warn(nil, "failed to read telegram PR threads", "err", err, "path", s.path)
		}
		return
	}
	var stored struct {
		IDs map[string]int `json:"ids"`
	}
	if err := json.Unmarshal(data, &stored); err != nil {
		logger.Warn(nil, "failed to parse telegram PR threads", "err", err, "path", s.path)
		return
	}
	if stored.IDs != nil {
		s.ids = stored.IDs
	}
}

func telegramPRThreadsPath() string {
	if config.GlobalConfig == nil || config.GlobalConfig.Root == "" {
		return ""
	}
	return filepath.Join(config.GlobalConfig.Root, "data", telegramPRThreadsFileName)
}

func telegramThreadNumberKey(repo string, number int) string {
	return fmt.Sprintf("%s|#%d", repo, number)
}

func telegramThreadTitleKey(repo, title string) string {
	return fmt.Sprintf("%s|%s", repo, gitverse.NormalizePRTitle(title))
}
