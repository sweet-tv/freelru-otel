package freelruotel

import "sync"

// instrumentedCache holds a cache instance with its name
type instrumentedCache struct {
	cache MetricsProvider
	name  string
}

// cacheRegistry manages a collection of instrumented caches with thread-safe access
type cacheRegistry struct {
	sync.RWMutex
	caches []instrumentedCache
}

// add appends a new cache to the registry
func (r *cacheRegistry) add(cache MetricsProvider, name string) {
	r.Lock()
	r.caches = append(r.caches, instrumentedCache{
		cache: cache,
		name:  name,
	})
	r.Unlock()
}

// forEach iterates over all caches with read lock
func (r *cacheRegistry) forEach(fn func(instrumentedCache)) {
	r.RLock()
	defer r.RUnlock()
	for _, ic := range r.caches {
		fn(ic)
	}
}

// reset clears all caches (used in tests)
func (r *cacheRegistry) reset() {
	r.Lock()
	r.caches = nil
	r.Unlock()
}
