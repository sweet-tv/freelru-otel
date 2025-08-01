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
	if err := registry.add(cache, name); err != nil {
		return err
	}

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
	// Create observers for all metrics
	hitObserver, err := meter.Int64ObservableCounter("cache.hit",
		metric.WithDescription("Number of cache hits"))
	if err != nil {
		return err
	}

	missObserver, err := meter.Int64ObservableCounter("cache.miss",
		metric.WithDescription("Number of cache misses"))
	if err != nil {
		return err
	}

	insertObserver, err := meter.Int64ObservableCounter("cache.insert",
		metric.WithDescription("Number of cache inserts"))
	if err != nil {
		return err
	}

	evictionObserver, err := meter.Int64ObservableCounter("cache.eviction",
		metric.WithDescription("Number of cache evictions"))
	if err != nil {
		return err
	}

	collisionObserver, err := meter.Int64ObservableCounter("cache.collision",
		metric.WithDescription("Number of cache collisions"))
	if err != nil {
		return err
	}

	removalObserver, err := meter.Int64ObservableCounter("cache.removal",
		metric.WithDescription("Number of cache removals"))
	if err != nil {
		return err
	}

	// Register single callback that observes all metrics at once
	_, err = meter.RegisterCallback(
		func(ctx context.Context, o metric.Observer) error {
			registry.forEach(func(name string, cache MetricsProvider) {
				metrics := cache.Metrics()
				attrs := metric.WithAttributes(attribute.String("cache_name", name))
				
				o.ObserveInt64(hitObserver, int64(metrics.Hits), attrs)
				o.ObserveInt64(missObserver, int64(metrics.Misses), attrs)
				o.ObserveInt64(insertObserver, int64(metrics.Inserts), attrs)
				o.ObserveInt64(evictionObserver, int64(metrics.Evictions), attrs)
				o.ObserveInt64(collisionObserver, int64(metrics.Collisions), attrs)
				o.ObserveInt64(removalObserver, int64(metrics.Removals), attrs)
			})
			return nil
		},
		hitObserver, missObserver, insertObserver, evictionObserver, collisionObserver, removalObserver,
	)

	return err
}

