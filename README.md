# freelru-otel

OpenTelemetry instrumentation for [elastic/go-freelru](https://github.com/elastic/go-freelru) LRU cache implementations.

## Features

- **Universal Instrumentation**: Works with both `freelru.SyncedLRU` and `freelru.ShardedLRU`
- **OpenTelemetry Integration**: Automatic metrics export for cache performance monitoring
- **Low Overhead**: Uses OpenTelemetry Observable Gauge callbacks for efficient metrics collection
- **Non-intrusive**: Metrics collection doesn't impact cache performance

## Installation

```bash
go get github.com/sweet-tv/freelru-otel
```

## Usage

### Basic Usage

```go
package main

import (
    "github.com/cespare/xxhash/v2"
    "github.com/elastic/go-freelru"
    "github.com/sweet-tv/freelru-otel"
)

func hashString(s string) uint32 {
    return uint32(xxhash.Sum64String(s))
}

func main() {
    // Create a SyncedLRU cache
    cache, err := freelru.NewSynced[string, string](100, hashString)
    if err != nil {
        panic(err)
    }

    // Instrument the cache (this starts collecting metrics)
    err = freelruotel.InstrumentCache(cache, "my_cache")
    if err != nil {
        panic(err)
    }

    // Use the cache normally
    cache.Add("key1", "value1")
    if value, ok := cache.Get("key1"); ok {
        // Metrics are automatically collected
    }
}
```

### Usage with ShardedLRU

```go
// Create a ShardedLRU cache for high concurrency
cache, err := freelru.NewSharded[string, string](1024, hashString)
if err != nil {
    panic(err)
}

// Instrument the cache
err = freelruotel.InstrumentCache(cache, "high_perf_cache")
if err != nil {
    panic(err)
}

// Use cache normally - metrics are collected automatically
cache.Add("key1", "value1")
cache.Get("key1")
```

### Using Custom MeterProvider

```go
// Create custom MeterProvider
provider := metric.NewMeterProvider()

// Create a cache
cache, err := freelru.NewSynced[string, string](100, hashString)
if err != nil {
    panic(err)
}

// Instrument the cache with custom MeterProvider
err = freelruotel.InstrumentCache(cache, "my_cache", 
    freelruotel.WithMeterProvider(provider))
if err != nil {
    panic(err)
}
```

## Exported Metrics

The instrumentation automatically exports the following OpenTelemetry metrics:

| Metric Name | Type | Description | Attributes |
|-------------|------|-------------|------------|
| `cache.hit` | Int64ObservableCounter | Number of cache hits | `cache_name` |
| `cache.miss` | Int64ObservableCounter | Number of cache misses | `cache_name` |
| `cache.insert` | Int64ObservableCounter | Number of cache inserts | `cache_name` |
| `cache.eviction` | Int64ObservableCounter | Number of cache evictions | `cache_name` |
| `cache.collision` | Int64ObservableCounter | Number of cache collisions | `cache_name` |
| `cache.removal` | Int64ObservableCounter | Number of cache removals | `cache_name` |

All metrics include the `cache_name` attribute to distinguish between different cache instances.

## Requirements

- Go 1.22+
- OpenTelemetry configured in your application

## License

This project is licensed under the MIT License.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.