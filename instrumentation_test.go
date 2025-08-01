package freelruotel

import (
	"context"
	"fmt"
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
			// Reset global state for test isolation
			resetForTesting()

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

func TestInstrumentMultipleCaches(t *testing.T) {
	// Reset global state for test isolation
	resetForTesting()

	// Create manual reader to collect metrics
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))

	// Create multiple caches
	cache1 := mustCreateLRUCache()
	cache2 := mustCreateSyncedCache()
	cache3 := mustCreateShardedCache()

	// Instrument all caches - this should not cause errors
	err := InstrumentCache(cache1, "cache1", WithMeterProvider(provider))
	if err != nil {
		t.Fatalf("Failed to instrument cache1: %v", err)
	}

	err = InstrumentCache(cache2, "cache2", WithMeterProvider(provider))
	if err != nil {
		t.Fatalf("Failed to instrument cache2: %v", err)
	}

	err = InstrumentCache(cache3, "cache3", WithMeterProvider(provider))
	if err != nil {
		t.Fatalf("Failed to instrument cache3: %v", err)
	}

	// Use the caches to generate different metrics
	cache1.Add("key1", "value1")
	cache1.Get("key1") // hit for cache1

	cache2.Add("key2", "value2")
	cache2.Get("missing") // miss for cache2

	cache3.Add("key3", "value3")
	cache3.Get("key3") // hit for cache3

	// Collect and verify metrics
	rm := &metricdata.ResourceMetrics{}
	err = reader.Collect(context.Background(), rm)
	if err != nil {
		t.Fatalf("Failed to collect metrics: %v", err)
	}

	if len(rm.ScopeMetrics) == 0 {
		t.Fatal("No metrics were exported")
	}

	metrics := rm.ScopeMetrics[0].Metrics
	if len(metrics) == 0 {
		t.Fatal("No cache metrics were exported")
	}

	// Verify all expected metrics are present
	expectedMetrics := []string{"cache.hit", "cache.miss", "cache.insert", "cache.eviction", "cache.collision", "cache.removal"}
	foundMetrics := make(map[string]*metricdata.Metrics)
	
	for i := range metrics {
		for _, expectedMetric := range expectedMetrics {
			if metrics[i].Name == expectedMetric {
				foundMetrics[expectedMetric] = &metrics[i]
			}
		}
	}

	// Check that all metrics are found
	for _, expectedMetric := range expectedMetrics {
		if foundMetrics[expectedMetric] == nil {
			t.Errorf("Expected metric %s not found", expectedMetric)
		}
	}

	expectedCaches := []string{"cache1", "cache2", "cache3"}

	// Verify each metric has data points for all cache names
	for metricName, metric := range foundMetrics {
		if metric == nil {
			continue
		}
		
		data := metric.Data.(metricdata.Sum[int64])
		
		// Check that we have observations for all caches
		if len(data.DataPoints) != len(expectedCaches) {
			t.Errorf("Metric %s: expected %d data points for different caches, got %d", 
				metricName, len(expectedCaches), len(data.DataPoints))
		}

		// Verify all expected cache names are present in this metric
		cacheNames := make(map[string]bool)
		for _, dp := range data.DataPoints {
			for _, attr := range dp.Attributes.ToSlice() {
				if attr.Key == "cache_name" {
					cacheNames[attr.Value.AsString()] = true
				}
			}
		}

		for _, expectedCache := range expectedCaches {
			if !cacheNames[expectedCache] {
				t.Errorf("Metric %s: expected cache name %s not found in metrics", metricName, expectedCache)
			}
		}
	}
}

func TestInstrumentCachesConcurrent(t *testing.T) {
	// Reset global state for test isolation
	resetForTesting()

	// Create manual reader to collect metrics
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))

	// Test concurrent calls to InstrumentCache
	const numGoroutines = 10
	const cachesPerGoroutine = 5

	errChan := make(chan error, numGoroutines)
	
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			for j := 0; j < cachesPerGoroutine; j++ {
				cache := mustCreateLRUCache()
				cacheName := fmt.Sprintf("cache_g%d_c%d", goroutineID, j)
				
				err := InstrumentCache(cache, cacheName, WithMeterProvider(provider))
				if err != nil {
					errChan <- fmt.Errorf("goroutine %d, cache %d: %v", goroutineID, j, err)
					return
				}
				
				// Use the cache to generate some metrics
				cache.Add("key", "value")
				cache.Get("key")
			}
			errChan <- nil
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		if err := <-errChan; err != nil {
			t.Fatal(err)
		}
	}

	// Collect metrics and verify all caches are tracked
	rm := &metricdata.ResourceMetrics{}
	err := reader.Collect(context.Background(), rm)
	if err != nil {
		t.Fatalf("Failed to collect metrics: %v", err)
	}

	if len(rm.ScopeMetrics) == 0 {
		t.Fatal("No metrics were exported")
	}

	// Should have metrics for all created caches
	expectedCacheCount := numGoroutines * cachesPerGoroutine
	
	// Verify all expected metrics are present and have correct number of data points
	expectedMetrics := []string{"cache.hit", "cache.miss", "cache.insert", "cache.eviction", "cache.collision", "cache.removal"}
	
	for _, expectedMetric := range expectedMetrics {
		var foundMetric *metricdata.Metrics
		for i := range rm.ScopeMetrics[0].Metrics {
			if rm.ScopeMetrics[0].Metrics[i].Name == expectedMetric {
				foundMetric = &rm.ScopeMetrics[0].Metrics[i]
				break
			}
		}

		if foundMetric == nil {
			t.Errorf("Metric %s not found", expectedMetric)
			continue
		}

		data := foundMetric.Data.(metricdata.Sum[int64])
		if len(data.DataPoints) != expectedCacheCount {
			t.Errorf("Metric %s: expected %d cache data points, got %d", 
				expectedMetric, expectedCacheCount, len(data.DataPoints))
		}

		// Verify all cache names are unique
		cacheNames := make(map[string]bool)
		for _, dp := range data.DataPoints {
			for _, attr := range dp.Attributes.ToSlice() {
				if attr.Key == "cache_name" {
					cacheName := attr.Value.AsString()
					if cacheNames[cacheName] {
						t.Errorf("Metric %s: duplicate cache name %s found", expectedMetric, cacheName)
					}
					cacheNames[cacheName] = true
				}
			}
		}

		if len(cacheNames) != expectedCacheCount {
			t.Errorf("Metric %s: expected %d unique cache names, got %d", 
				expectedMetric, expectedCacheCount, len(cacheNames))
		}
	}
}

func TestInstrumentCacheDuplicateName(t *testing.T) {
	// Reset global state for test isolation
	resetForTesting()

	// Create manual reader to collect metrics
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))

	// Create two different caches
	cache1 := mustCreateLRUCache()
	cache2 := mustCreateSyncedCache()

	// First cache should succeed
	err := InstrumentCache(cache1, "duplicate_name", WithMeterProvider(provider))
	if err != nil {
		t.Fatalf("First cache should not fail: %v", err)
	}

	// Second cache with same name should fail
	err = InstrumentCache(cache2, "duplicate_name", WithMeterProvider(provider))
	if err == nil {
		t.Fatal("Expected error when adding cache with duplicate name")
	}

	expectedError := "cache with name 'duplicate_name' already exists"
	if err.Error() != expectedError {
		t.Errorf("Expected error message '%s', got '%s'", expectedError, err.Error())
	}
}
