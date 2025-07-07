package freelruotel

import (
	"context"

	"github.com/elastic/go-freelru"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// MetricsProvider is an interface for any cache implementation that can provide metrics.
// Both freelru.SyncedLRU and freelru.ShardedLRU implement this interface.
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

// InstrumentCache registers OpenTelemetry Observable Gauge metrics for a cache
// that implements the MetricsProvider interface.
func InstrumentCache(cache MetricsProvider, name string, opts ...Option) error {
	cfg := &config{
		meterProvider: otel.GetMeterProvider(),
	}

	// Apply options
	for _, opt := range opts {
		opt(cfg)
	}

	meter := cfg.meterProvider.Meter("github.com/sweet-tv/freelru-otel")
	if meter == nil {
		return nil
	}

	commonAttrs := []attribute.KeyValue{
		attribute.String("cache_name", name),
	}

	// Register all cache metrics
	if err := registerMetric(meter, "cache.hit", "Number of cache hits",
		func(ctx context.Context, o metric.Int64Observer) error {
			metrics := cache.Metrics()
			o.Observe(int64(metrics.Hits), metric.WithAttributes(commonAttrs...))
			return nil
		}); err != nil {
		return err
	}

	if err := registerMetric(meter, "cache.miss", "Number of cache misses",
		func(ctx context.Context, o metric.Int64Observer) error {
			metrics := cache.Metrics()
			o.Observe(int64(metrics.Misses), metric.WithAttributes(commonAttrs...))
			return nil
		}); err != nil {
		return err
	}

	if err := registerMetric(meter, "cache.insert", "Number of cache inserts",
		func(ctx context.Context, o metric.Int64Observer) error {
			metrics := cache.Metrics()
			o.Observe(int64(metrics.Inserts), metric.WithAttributes(commonAttrs...))
			return nil
		}); err != nil {
		return err
	}

	if err := registerMetric(meter, "cache.eviction", "Number of cache evictions",
		func(ctx context.Context, o metric.Int64Observer) error {
			metrics := cache.Metrics()
			o.Observe(int64(metrics.Evictions), metric.WithAttributes(commonAttrs...))
			return nil
		}); err != nil {
		return err
	}

	if err := registerMetric(meter, "cache.collision", "Number of cache collisions",
		func(ctx context.Context, o metric.Int64Observer) error {
			metrics := cache.Metrics()
			o.Observe(int64(metrics.Collisions), metric.WithAttributes(commonAttrs...))
			return nil
		}); err != nil {
		return err
	}

	if err := registerMetric(meter, "cache.removal", "Number of cache removals",
		func(ctx context.Context, o metric.Int64Observer) error {
			metrics := cache.Metrics()
			o.Observe(int64(metrics.Removals), metric.WithAttributes(commonAttrs...))
			return nil
		}); err != nil {
		return err
	}

	return nil
}

// registerMetric is a helper function to register an individual metric and handle errors.
func registerMetric(meter metric.Meter, name, description string, callback metric.Int64Callback) error {
	_, err := meter.Int64ObservableCounter(name,
		metric.WithDescription(description),
		metric.WithInt64Callback(callback))
	return err
}