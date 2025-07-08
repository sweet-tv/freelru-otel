package freelruotel

import (
	"context"
	"testing"

	"github.com/cespare/xxhash/v2"
	"github.com/elastic/go-freelru"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// hashStringXXHASH is a hash function using xxhash for testing
func hashStringXXHASH(s string) uint32 {
	return uint32(xxhash.Sum64String(s))
}

func TestInstrumentCache(t *testing.T) {
	testCases := []struct {
		name  string
		cache MetricsProvider
	}{
		{
			name:  "LRU",
			cache: mustCreateLRUCache(),
		},
		{
			name:  "ShardedLRU",
			cache: mustCreateShardedCache(),
		},
		{
			name:  "SyncedLRU",
			cache: mustCreateSyncedCache(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create manual reader to collect metrics
			reader := metric.NewManualReader()
			provider := metric.NewMeterProvider(metric.WithReader(reader))

			// Instrument the cache
			err := InstrumentCache(tc.cache, "test_cache", WithMeterProvider(provider))
			if err != nil {
				t.Fatalf("Failed to instrument cache: %v", err)
			}

			// Use the cache to generate metrics
			if lruCache, ok := tc.cache.(*freelru.LRU[string, string]); ok {
				lruCache.Add("key1", "value1")
				lruCache.Add("key2", "value2")
				lruCache.Get("key1") // hit
				lruCache.Get("miss") // miss
			} else if syncedCache, ok := tc.cache.(*freelru.SyncedLRU[string, string]); ok {
				syncedCache.Add("key1", "value1")
				syncedCache.Add("key2", "value2")
				syncedCache.Get("key1") // hit
				syncedCache.Get("miss") // miss
			} else if shardedCache, ok := tc.cache.(*freelru.ShardedLRU[string, string]); ok {
				shardedCache.Add("key1", "value1")
				shardedCache.Add("key2", "value2")
				shardedCache.Get("key1") // hit
				shardedCache.Get("miss") // miss
			}

			// Collect and verify metrics are exported
			rm := &metricdata.ResourceMetrics{}
			err = reader.Collect(context.Background(), rm)
			if err != nil {
				t.Fatalf("Failed to collect metrics: %v", err)
			}

			if len(rm.ScopeMetrics) == 0 {
				t.Fatal("No metrics were exported")
			}

			// Verify we have cache metrics
			if len(rm.ScopeMetrics[0].Metrics) == 0 {
				t.Fatal("No cache metrics were exported")
			}

			// Verify cache.hit and cache.miss metrics are present
			metrics := rm.ScopeMetrics[0].Metrics
			var hitMetric, missMetric *metricdata.Metrics

			for i := range metrics {
				switch metrics[i].Name {
				case "cache.hit":
					hitMetric = &metrics[i]
				case "cache.miss":
					missMetric = &metrics[i]
				}
			}

			if hitMetric == nil {
				t.Error("cache.hit metric not found")
			}
			if missMetric == nil {
				t.Error("cache.miss metric not found")
			}
		})
	}
}

func mustCreateShardedCache() *freelru.ShardedLRU[string, string] {
	cache, err := freelru.NewSharded[string, string](10, hashStringXXHASH)
	if err != nil {
		panic(err)
	}
	return cache
}

func mustCreateLRUCache() *freelru.LRU[string, string] {
	cache, err := freelru.New[string, string](10, hashStringXXHASH)
	if err != nil {
		panic(err)
	}
	return cache
}

func mustCreateSyncedCache() *freelru.SyncedLRU[string, string] {
	cache, err := freelru.NewSynced[string, string](10, hashStringXXHASH)
	if err != nil {
		panic(err)
	}
	return cache
}
