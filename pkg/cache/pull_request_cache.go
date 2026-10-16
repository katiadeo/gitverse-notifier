package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gitverse-notifier/pkg/config"
	"gitverse-notifier/pkg/integrations/gitverse"
	"gitverse-notifier/pkg/logger"

	gocache "github.com/patrickmn/go-cache"
)

const (
	cacheCleanupPeriod = 30 * time.Minute
	prNumbersFileName  = "pr-numbers.json"
)

type PullRequestCache interface {
	// Во многих событиях есть только заголовок PR, поэтому дополнительно кэширует номер
	GetByTitle(repo, title string, number int) (*gitverse.PullRequest, bool)
	GetPRNumber(repo, title string) int
	Set(repo, title string, number int, pr *gitverse.PullRequest)
}

type MemoryPullRequestCache struct {
	cache *gocache.Cache
	mu    sync.Mutex
	path  string
}

type persistedPRNumbers struct {
	Numbers map[string]int `json:"numbers"`
}

func NewMemoryPullRequestCache() *MemoryPullRequestCache {
	ttl := time.Duration(config.GlobalConfig.CacheTTL)
	s := &MemoryPullRequestCache{
		cache: gocache.New(ttl*time.Minute, cacheCleanupPeriod),
		path:  prNumbersPath(),
	}
	s.loadNumbers()
	return s
}

func (s *MemoryPullRequestCache) GetByTitle(repo, title string, number int) (pr *gitverse.PullRequest, ok bool) {
	remembered := s.GetPRNumber(repo, title)
	if number <= 0 {
		number = remembered
	}
	if number <= 0 {
		return nil, false
	}

	value, ok := s.cache.Get(prCacheKey(repo, number))
	if ok {
		pr, ok = value.(*gitverse.PullRequest)
		if ok {
			return pr, true
		}
	}
	if remembered > 0 || number > 0 {
		return gitverse.NewStubPullRequest(repo, title, number), true
	}
	return nil, false
}

func (s *MemoryPullRequestCache) Set(repo, title string, number int, pr *gitverse.PullRequest) {
	if pr == nil || number <= 0 {
		return
	}
	s.cache.Set(prCacheKey(repo, number), pr, gocache.DefaultExpiration)
	normalized := gitverse.NormalizePRTitle(title)
	if normalized != "" {
		s.cache.Set(prNumberCacheKey(repo, normalized), number, gocache.NoExpiration)
	}
	if title != "" && title != normalized {
		s.cache.Set(prNumberCacheKey(repo, title), number, gocache.NoExpiration)
	}
	if issueKey := prIssueKeyCacheKey(repo, title); issueKey != "" {
		s.cache.Set(issueKey, number, gocache.NoExpiration)
	}
	s.persistNumbers()
}

func (s *MemoryPullRequestCache) GetPRNumber(repo, title string) int {
	value, ok := s.cache.Get(prNumberCacheKey(repo, gitverse.NormalizePRTitle(title)))
	if !ok {
		value, ok = s.cache.Get(prNumberCacheKey(repo, title))
	}
	if !ok {
		if issueKey := prIssueKeyCacheKey(repo, title); issueKey != "" {
			value, ok = s.cache.Get(issueKey)
		}
	}
	if !ok {
		return 0
	}

	if number, ok := value.(int); ok {
		return number
	}

	return 0
}

func (s *MemoryPullRequestCache) persistNumbers() {
	if s.path == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	out := persistedPRNumbers{Numbers: map[string]int{}}
	for key, item := range s.cache.Items() {
		number, ok := item.Object.(int)
		if !ok || number <= 0 {
			continue
		}
		out.Numbers[key] = number
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		logger.Warn(nil, "failed to encode PR number cache", "err", err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		logger.Warn(nil, "failed to create PR number cache dir", "err", err, "path", s.path)
		return
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		logger.Warn(nil, "failed to write PR number cache", "err", err, "path", tmp)
		return
	}
	if err := os.Rename(tmp, s.path); err != nil {
		logger.Warn(nil, "failed to replace PR number cache", "err", err, "path", s.path)
		_ = os.Remove(tmp)
	}
}

func (s *MemoryPullRequestCache) loadNumbers() {
	if s.path == "" {
		return
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warn(nil, "failed to read PR number cache", "err", err, "path", s.path)
		}
		return
	}
	var stored persistedPRNumbers
	if err := json.Unmarshal(data, &stored); err != nil {
		logger.Warn(nil, "failed to parse PR number cache", "err", err, "path", s.path)
		return
	}
	for key, number := range stored.Numbers {
		if number > 0 && key != "" {
			s.cache.Set(key, number, gocache.NoExpiration)
			if repo, title, ok := splitPRNumberCacheKey(key); ok {
				if issueKey := prIssueKeyCacheKey(repo, title); issueKey != "" {
					s.cache.Set(issueKey, number, gocache.NoExpiration)
				}
			}
		}
	}
}

func splitPRNumberCacheKey(key string) (repo, title string, ok bool) {
	if strings.Contains(key, "|key:") {
		return "", "", false
	}
	repo, title, ok = strings.Cut(key, "|")
	return repo, title, ok && title != ""
}

func prNumbersPath() string {
	if config.GlobalConfig == nil {
		return ""
	}
	root := config.GlobalConfig.Root
	if root == "" {
		return ""
	}
	return filepath.Join(root, "data", prNumbersFileName)
}

func prCacheKey(repo string, number int) string {
	return fmt.Sprintf("%s#%d", repo, number)
}

func prNumberCacheKey(repo, title string) string {
	return fmt.Sprintf("%s|%s", repo, title)
}

func prIssueKeyCacheKey(repo, title string) string {
	key := gitverse.IssueKeyFromTitle(title)
	if key == "" {
		return ""
	}
	return fmt.Sprintf("%s|key:%s", repo, key)
}
