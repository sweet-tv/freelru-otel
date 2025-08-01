package freelruotel

import (
	"context"
	"sync/atomic"

	"github.com/elastic/go-freelru"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// version is the current version of the instrumentation library.
var version = "v0.1.0-dev"

// Global state for tracking multiple cache instances
var (
	registry          = &cacheRegistry{}
	metricsRegistered atomic.Bool
)

// MetricsProvider is an interface for freelru cache implementations that can provide metrics.
// freelru.LRU, freelru.SyncedLRU and freelru.ShardedLRU implement this interface.
type MetricsProvider interface {
	Metrics() freelru.Metrics
}

// Option is a functional option for configuring cache instrumentation.
type Option func(*config)

type config struct {
	meterProvider metric.MeterProvider
}

// WithMeterProvider sets a custom MeterProvider for the instrumentation.
func WithMeterProvider(provider metric.MeterProvider) Option {
	return func(c *config) {
		c.meterProvider = provider
	}
}

// InstrumentCache registers OpenTelemetry Observable Counter metrics of any instance of freelru cache.
func InstrumentCache(cache MetricsProvider, name string, opts ...Option) error {
	cfg := &config{
		meterProvider: otel.GetMeterProvider(),
	}

	// Apply options
	for _, opt := range opts {
		opt(cfg)
	}

	// Add the cache to our global registry
	registry.add(cache, name)

	// Register metrics only once using atomic compare-and-swap
	if !metricsRegistered.CompareAndSwap(false, true) {
		return nil
	}

	meter := cfg.meterProvider.Meter("github.com/sweet-tv/freelru-otel",
		metric.WithInstrumentationVersion(version))
	if meter == nil {
		return nil
	}

	return registerAllMetrics(meter)
}

// registerAllMetrics registers all cache metrics with the provided meter
func registerAllMetrics(meter metric.Meter) error {
	// Register all cache metrics with callbacks that iterate over all caches
	if err := registerMetric(meter, "cache.hit", "Number of cache hits",
		func(ctx context.Context, o metric.Int64Observer) error {
			registry.forEach(func(ic instrumentedCache) {
				metrics := ic.cache.Metrics()
				attrs := []attribute.KeyValue{attribute.String("cache_name", ic.name)}
				o.Observe(int64(metrics.Hits), metric.WithAttributes(attrs...))
			})
			return nil
		}); err != nil {
		return err
	}

	if err := registerMetric(meter, "cache.miss", "Number of cache misses",
		func(ctx context.Context, o metric.Int64Observer) error {
			registry.forEach(func(ic instrumentedCache) {
				metrics := ic.cache.Metrics()
				attrs := []attribute.KeyValue{attribute.String("cache_name", ic.name)}
				o.Observe(int64(metrics.Misses), metric.WithAttributes(attrs...))
			})
			return nil
		}); err != nil {
		return err
	}

	if err := registerMetric(meter, "cache.insert", "Number of cache inserts",
		func(ctx context.Context, o metric.Int64Observer) error {
			registry.forEach(func(ic instrumentedCache) {
				metrics := ic.cache.Metrics()
				attrs := []attribute.KeyValue{attribute.String("cache_name", ic.name)}
				o.Observe(int64(metrics.Inserts), metric.WithAttributes(attrs...))
			})
			return nil
		}); err != nil {
		return err
	}

	if err := registerMetric(meter, "cache.eviction", "Number of cache evictions",
		func(ctx context.Context, o metric.Int64Observer) error {
			registry.forEach(func(ic instrumentedCache) {
				metrics := ic.cache.Metrics()
				attrs := []attribute.KeyValue{attribute.String("cache_name", ic.name)}
				o.Observe(int64(metrics.Evictions), metric.WithAttributes(attrs...))
			})
			return nil
		}); err != nil {
		return err
	}

	if err := registerMetric(meter, "cache.collision", "Number of cache collisions",
		func(ctx context.Context, o metric.Int64Observer) error {
			registry.forEach(func(ic instrumentedCache) {
				metrics := ic.cache.Metrics()
				attrs := []attribute.KeyValue{attribute.String("cache_name", ic.name)}
				o.Observe(int64(metrics.Collisions), metric.WithAttributes(attrs...))
			})
			return nil
		}); err != nil {
		return err
	}

	if err := registerMetric(meter, "cache.removal", "Number of cache removals",
		func(ctx context.Context, o metric.Int64Observer) error {
			registry.forEach(func(ic instrumentedCache) {
				metrics := ic.cache.Metrics()
				attrs := []attribute.KeyValue{attribute.String("cache_name", ic.name)}
				o.Observe(int64(metrics.Removals), metric.WithAttributes(attrs...))
			})
			return nil
		}); err != nil {
		return err
	}

	return nil
}

func registerMetric(meter metric.Meter, name, description string, callback metric.Int64Callback) error {
	_, err := meter.Int64ObservableCounter(name,
		metric.WithDescription(description),
		metric.WithInt64Callback(callback))
	return err
}
