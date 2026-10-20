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
	MessageID  int                  `json:"message_id,omitempty"`
	RootID     int                  `json:"root_id,omitempty"`
	CommentID  int                  `json:"comment_id,omitempty"`
	ExtraIDs   []int                `json:"extra_ids,omitempty"`
	Repository string               `json:"repository"`
	Number     int                  `json:"number"`
	Title      string               `json:"title"`
	URL        string               `json:"url"`
	PRAuthor   events.Actor         `json:"pr_author"`
	Authors    []events.Actor       `json:"authors,omitempty"`
	Comments   []events.CommentInfo `json:"comments,omitempty"`
	Approvals  []events.CommentInfo `json:"approvals,omitempty"`
	Rejections []events.CommentInfo `json:"rejections,omitempty"`
	Reviewers  []events.Actor       `json:"reviewers,omitempty"`
	MergedBy   events.Actor         `json:"merged_by,omitempty"`
	Rich       bool                 `json:"rich,omitempty"`
	Opened     bool                 `json:"opened,omitempty"`
	Merged     bool                 `json:"merged,omitempty"`
	Closed     bool                 `json:"closed,omitempty"`
}

func (c *TelegramPRCard) ResolvedMessageID() int {
	if c == nil {
		return 0
	}
	if c.MessageID > 0 {
		return c.MessageID
	}
	if c.CommentID > 0 {
		return c.CommentID
	}
	return c.RootID
}

func (c *TelegramPRCard) SetMessageID(id int) {
	if c == nil {
		return
	}
	c.MessageID = id
	c.RootID = 0
	c.CommentID = 0
	c.ExtraIDs = nil
}

func (c *TelegramPRCard) LegacyMessageIDs(keep int) []int {
	if c == nil {
		return nil
	}
	seen := map[int]struct{}{}
	var out []int
	add := func(id int) {
		if id <= 0 || id == keep {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	add(c.RootID)
	add(c.CommentID)
	for _, id := range c.ExtraIDs {
		add(id)
	}
	if c.MessageID > 0 && c.MessageID != keep {
		add(c.MessageID)
	}
	return out
}

func (c *TelegramPRCard) normalize() {
	if c == nil {
		return
	}
	if c.MessageID <= 0 {
		c.MessageID = c.CommentID
		if c.MessageID <= 0 {
			c.MessageID = c.RootID
		}
	}
	c.RootID = 0
	c.CommentID = 0
	c.ExtraIDs = nil
	if len(c.Comments) == 0 && len(c.Authors) > 0 {
		c.Comments = authorsToComments(c.Authors)
	}
	if len(c.Comments) > 0 {
		c.Authors = nil
	}
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
	for _, key := range telegramThreadKeys(stored.Repository, stored.Title, stored.Number) {
		if existing := s.cards[key]; existing != nil {
			stored = mergePRCards(existing, stored)
		}
	}
	for _, key := range telegramThreadKeys(stored.Repository, stored.Title, stored.Number) {
		s.cards[key] = stored
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
		for _, key := range telegramThreadKeys(card.Repository, card.Title, card.Number) {
			delete(s.cards, key)
		}
	}
	for _, key := range telegramThreadKeys(repo, title, number) {
		delete(s.cards, key)
	}
	s.persistLocked()
}

func (s *TelegramPRThreads) lookupLocked(repo, title string, number int) *TelegramPRCard {
	for _, key := range telegramThreadKeys(repo, title, number) {
		if card := s.cards[key]; card != nil {
			return card
		}
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
		s.cards = map[string]*TelegramPRCard{}
		seen := map[*TelegramPRCard]struct{}{}
		for _, card := range stored.Cards {
			if card == nil {
				continue
			}
			if _, ok := seen[card]; ok {
				continue
			}
			card.normalize()
			seen[card] = struct{}{}
			for _, key := range telegramThreadKeys(card.Repository, card.Title, card.Number) {
				s.cards[key] = card
			}
		}
		return
	}
	for key, id := range stored.IDs {
		if id > 0 {
			s.cards[key] = &TelegramPRCard{MessageID: id}
		}
	}
}

func telegramPRThreadsPath() string {
	if config.GlobalConfig == nil || config.GlobalConfig.Root == "" {
		return ""
	}
	return filepath.Join(config.GlobalConfig.Root, "data", telegramPRThreadsFileName)
}

func telegramThreadKeys(repo, title string, number int) []string {
	keys := make([]string, 0, 3)
	if number > 0 {
		keys = append(keys, telegramThreadNumberKey(repo, number))
	}
	for _, issue := range gitverse.IssueKeysFromTitle(title) {
		keys = append(keys, telegramThreadIssueKey(repo, issue))
	}
	if normalized := gitverse.NormalizePRTitle(title); normalized != "" {
		keys = append(keys, telegramThreadTitleKey(repo, title))
	}
	return keys
}

func telegramThreadNumberKey(repo string, number int) string {
	return fmt.Sprintf("%s|#%d", repo, number)
}

func telegramThreadIssueKey(repo, issue string) string {
	return fmt.Sprintf("%s|key:%s", repo, strings.ToUpper(strings.TrimSpace(issue)))
}

func telegramThreadTitleKey(repo, title string) string {
	return fmt.Sprintf("%s|%s", repo, gitverse.NormalizePRTitle(title))
}

func CommentLockKey(repo, title string, number int) string {
	keys := telegramThreadKeys(repo, title, number)
	if len(keys) > 0 {
		return keys[0]
	}
	return strings.TrimSpace(repo)
}

func mergePRCards(existing, incoming *TelegramPRCard) *TelegramPRCard {
	if existing == nil {
		return clonePRCard(incoming)
	}
	out := clonePRCard(existing)
	if incoming == nil {
		return out
	}
	if id := incoming.ResolvedMessageID(); id > 0 {
		out.SetMessageID(id)
	}
	if incoming.Opened {
		out.Opened = true
	}
	if incoming.Rich {
		out.Rich = true
	}
	if incoming.Number > 0 {
		out.Number = incoming.Number
	}
	if strings.TrimSpace(incoming.Title) != "" {
		out.Title = incoming.Title
	}
	if strings.TrimSpace(incoming.URL) != "" {
		out.URL = incoming.URL
	}
	if incoming.PRAuthor != (events.Actor{}) {
		out.PRAuthor = incoming.PRAuthor
	}
	out.Comments = append([]events.CommentInfo(nil), incoming.Comments...)
	if len(out.Comments) == 0 && len(incoming.Authors) > 0 {
		out.Comments = authorsToComments(incoming.Authors)
	}
	if len(out.Comments) == 0 {
		out.Comments = nil
	}
	out.Authors = nil
	if len(incoming.Approvals) > 0 {
		out.Approvals = append([]events.CommentInfo(nil), incoming.Approvals...)
	}
	if len(incoming.Rejections) > 0 {
		out.Rejections = append([]events.CommentInfo(nil), incoming.Rejections...)
	}
	out.Reviewers = append([]events.Actor(nil), incoming.Reviewers...)
	if incoming.MergedBy != (events.Actor{}) {
		out.MergedBy = incoming.MergedBy
	}
	if incoming.Merged {
		out.Merged = true
	}
	if incoming.Closed {
		out.Closed = true
	}
	return out
}

func clonePRCard(card *TelegramPRCard) *TelegramPRCard {
	if card == nil {
		return nil
	}
	out := *card
	out.normalize()
	if card.Comments != nil {
		out.Comments = append([]events.CommentInfo(nil), card.Comments...)
	} else if len(out.Comments) == 0 && len(card.Authors) > 0 {
		out.Comments = authorsToComments(card.Authors)
	}
	out.Authors = nil
	out.ExtraIDs = nil
	if card.Approvals != nil {
		out.Approvals = append([]events.CommentInfo(nil), card.Approvals...)
	}
	if card.Rejections != nil {
		out.Rejections = append([]events.CommentInfo(nil), card.Rejections...)
	}
	if card.Reviewers != nil {
		out.Reviewers = append([]events.Actor(nil), card.Reviewers...)
	}
	return &out
}

func authorsToComments(authors []events.Actor) []events.CommentInfo {
	out := make([]events.CommentInfo, 0, len(authors))
	for _, author := range authors {
		if author == (events.Actor{}) {
			continue
		}
		out = append(out, events.CommentInfo{Author: author})
	}
	return out
}
