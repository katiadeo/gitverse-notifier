package cache

import (
	"fmt"
	"gitverse-notifier/pkg/config"
	"gitverse-notifier/pkg/integrations/gitverse"
	"time"

	gocache "github.com/patrickmn/go-cache"
)

const cacheCleanupPeriod = 30 * time.Minute

type PullRequestCache interface {
	// Во многих событиях есть только заголовок PR, поэтому дополнительно кэширует номер
	GetByTitle(repo, title string, number int) (*gitverse.PullRequest, bool)
	GetPRNumber(repo, title string) int
	Set(repo, title string, number int, pr *gitverse.PullRequest)
}

type MemoryPullRequestCache struct {
	cache *gocache.Cache
}

func NewMemoryPullRequestCache() *MemoryPullRequestCache {
	ttl := time.Duration(config.GlobalConfig.CacheTTL)
	return &MemoryPullRequestCache{
		cache: gocache.New(ttl*time.Minute, cacheCleanupPeriod),
	}
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
	if remembered > 0 {
		return gitverse.NewStubPullRequest(repo, title, number), true
	}
	return nil, false
}

func (s *MemoryPullRequestCache) Set(repo, title string, number int, pr *gitverse.PullRequest) {
	if pr == nil || number <= 0 {
		return
	}
	s.cache.Set(prCacheKey(repo, number), pr, gocache.DefaultExpiration)
	s.cache.Set(prNumberCacheKey(repo, gitverse.NormalizePRTitle(title)), number, 48*time.Hour)
	if title != gitverse.NormalizePRTitle(title) {
		s.cache.Set(prNumberCacheKey(repo, title), number, 48*time.Hour)
	}
}

func (s *MemoryPullRequestCache) GetPRNumber(repo, title string) int {
	value, ok := s.cache.Get(prNumberCacheKey(repo, gitverse.NormalizePRTitle(title)))
	if !ok {
		value, ok = s.cache.Get(prNumberCacheKey(repo, title))
	}
	if !ok {
		return 0
	}

	if number, ok := value.(int); ok {
		return number
	}

	return 0
}

func prCacheKey(repo string, number int) string {
	return fmt.Sprintf("%s#%d", repo, number)
}

func prNumberCacheKey(repo, title string) string {
	return fmt.Sprintf("%s|%s", repo, title)
}
