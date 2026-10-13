import (
	"sync"
	"time"
)

// GitHubProfileCache remembers GitHub lookups by address for a while, at
// most 256 of them, forgetting the oldest first.
type GitHubProfileCache struct {
	lock    sync.Mutex
	entries map[string]GitHubProfileEntry
	order   []string
}

// Lookup answers a remembered lookup that has not expired.
func (cache *GitHubProfileCache) Lookup(address string) (map[string]any, bool) {
	cache.lock.Lock()
	defer cache.lock.Unlock()
	entry, found := cache.entries[address]
	if !found {
		return nil, false
	}
	if entry.ExpiresAt.After(time.Now()) {
		return entry.Value, true
	}
	cache.forget(address)
	return nil, false
}

// Remember keeps a lookup for ttl and answers it.
func (cache *GitHubProfileCache) Remember(address string, value map[string]any, ttl time.Duration) map[string]any {
	cache.lock.Lock()
	defer cache.lock.Unlock()
	if cache.entries == nil {
		cache.entries = map[string]GitHubProfileEntry{}
	}
	if len(cache.entries) >= 256 && len(cache.order) > 0 {
		cache.forget(cache.order[0])
	}
	if _, known := cache.entries[address]; !known {
		cache.order = append(cache.order, address)
	}
	cache.entries[address] = GitHubProfileEntry{ExpiresAt: time.Now().Add(ttl), Value: value}
	return value
}

func (cache *GitHubProfileCache) forget(address string) {
	delete(cache.entries, address)
	for index, known := range cache.order {
		if known == address {
			cache.order = append(cache.order[:index], cache.order[index+1:]...)
			break
		}
	}
}
