package freelruotel

import (
	"fmt"
	"sync"
)

// cacheRegistry manages a collection of instrumented caches with thread-safe access
type cacheRegistry struct {
	sync.RWMutex
	caches map[string]MetricsProvider
}

// add stores a new cache in the registry, returning error if name already exists
func (r *cacheRegistry) add(cache MetricsProvider, name string) error {
	r.Lock()
	defer r.Unlock()
	
	if r.caches == nil {
		r.caches = make(map[string]MetricsProvider)
	}
	
	if _, exists := r.caches[name]; exists {
		return fmt.Errorf("cache with name '%s' already exists", name)
	}
	
	r.caches[name] = cache
	return nil
}

// forEach iterates over all caches
func (r *cacheRegistry) forEach(fn func(string, MetricsProvider)) {
	r.RLock()
	defer r.RUnlock()
	for name, cache := range r.caches {
		fn(name, cache)
	}
}

// reset clears all caches (used in tests)
func (r *cacheRegistry) reset() {
	r.Lock()
	defer r.Unlock()
	r.caches = make(map[string]MetricsProvider)
}

// resetForTesting resets both registry and metrics registration for tests
func resetForTesting() {
	registry.reset()
	metricsOnce = sync.Once{}
}
