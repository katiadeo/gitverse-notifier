package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gitverse-notifier/pkg/config"
	"gitverse-notifier/pkg/events"
	"gitverse-notifier/pkg/integrations/gitverse"
	"gitverse-notifier/pkg/logger"
)

const telegramPRThreadsFileName = "telegram-pr-threads.json"

type TelegramPRCard struct {
	RootID     int            `json:"root_id"`
	CommentID  int            `json:"comment_id"`
	ExtraIDs   []int          `json:"extra_ids,omitempty"`
	Repository string         `json:"repository"`
	Number     int            `json:"number"`
	Title      string         `json:"title"`
	URL        string         `json:"url"`
	PRAuthor   events.Actor   `json:"pr_author"`
	Authors    []events.Actor `json:"authors"`
	Rich       bool           `json:"rich,omitempty"`
}

type TelegramPRThreads struct {
	mu    sync.Mutex
	path  string
	cards map[string]*TelegramPRCard
}

func NewTelegramPRThreads() *TelegramPRThreads {
	s := &TelegramPRThreads{
		path:  telegramPRThreadsPath(),
		cards: map[string]*TelegramPRCard{},
	}
	s.load()
	return s
}

func (s *TelegramPRThreads) Get(repo, title string, number int) *TelegramPRCard {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	return clonePRCard(s.lookupLocked(repo, title, number))
}

func (s *TelegramPRThreads) Put(card *TelegramPRCard) {
	if s == nil || card == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	stored := clonePRCard(card)
	if stored.Number > 0 {
		s.cards[telegramThreadNumberKey(stored.Repository, stored.Number)] = stored
	}
	if strings.TrimSpace(stored.Title) != "" {
		s.cards[telegramThreadTitleKey(stored.Repository, stored.Title)] = stored
	}
	s.persistLocked()
}

func (s *TelegramPRThreads) Remove(repo, title string, number int) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	card := s.lookupLocked(repo, title, number)
	if card != nil {
		if card.Number > 0 {
			delete(s.cards, telegramThreadNumberKey(card.Repository, card.Number))
		}
		if strings.TrimSpace(card.Title) != "" {
			delete(s.cards, telegramThreadTitleKey(card.Repository, card.Title))
		}
	}
	if number > 0 {
		delete(s.cards, telegramThreadNumberKey(repo, number))
	}
	if strings.TrimSpace(title) != "" {
		delete(s.cards, telegramThreadTitleKey(repo, title))
	}
	s.persistLocked()
}

func (s *TelegramPRThreads) lookupLocked(repo, title string, number int) *TelegramPRCard {
	if number > 0 {
		if card := s.cards[telegramThreadNumberKey(repo, number)]; card != nil {
			return card
		}
	}
	if strings.TrimSpace(title) != "" {
		return s.cards[telegramThreadTitleKey(repo, title)]
	}
	return nil
}

func (s *TelegramPRThreads) persistLocked() {
	if s.path == "" {
		return
	}
	out := struct {
		Cards map[string]*TelegramPRCard `json:"cards"`
	}{Cards: s.cards}
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
		Cards map[string]*TelegramPRCard `json:"cards"`
		IDs   map[string]int             `json:"ids"`
	}
	if err := json.Unmarshal(data, &stored); err != nil {
		logger.Warn(nil, "failed to parse telegram PR threads", "err", err, "path", s.path)
		return
	}
	if stored.Cards != nil {
		s.cards = stored.Cards
		return
	}
	for key, id := range stored.IDs {
		if id > 0 {
			s.cards[key] = &TelegramPRCard{RootID: id}
		}
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

func clonePRCard(card *TelegramPRCard) *TelegramPRCard {
	if card == nil {
		return nil
	}
	out := *card
	if card.Authors != nil {
		out.Authors = append([]events.Actor(nil), card.Authors...)
	}
	if card.ExtraIDs != nil {
		out.ExtraIDs = append([]int(nil), card.ExtraIDs...)
	}
	return &out
}
