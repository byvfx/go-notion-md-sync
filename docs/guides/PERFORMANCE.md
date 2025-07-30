# Performance Optimization Guide

## Overview

This guide covers performance optimizations for notion-md-sync that can improve sync speeds by up to 26% based on extensive testing.

## Quick Start

For optimal performance out of the box, notion-md-sync v0.15.0+ automatically detects and uses the best worker configuration:

```bash
# Default configuration will use optimized settings
notion-md-sync pull
```

## Performance Configuration

### Basic Configuration

Add these settings to your `config.yaml`:

```yaml
# Performance settings (v0.15.0+)
performance:
  workers: 0  # 0 = auto-detect (recommended)
```

### Advanced Configuration

For fine-tuning performance:

```yaml
performance:
  # Worker count
  # - 0: Auto-detect (30 for large, 20 for medium, page count for small)
  # - 30: Maximum performance for large workspaces
  # - Custom: Set specific number based on your needs
  workers: 30
  
  # Multi-client mode (experimental)
  # Uses multiple HTTP clients for round-robin request distribution
  use_multi_client: false
  
  # Number of HTTP clients when multi-client is enabled
  client_count: 3
```

## Performance Benchmarks

Based on testing with a 14-page Notion workspace:

| Configuration | Time | Pages/Second | Improvement |
|---------------|------|--------------|-------------|
| Baseline (v0.14.0) | 95.8s | 0.15 | - |
| Optimized (30 workers) | **70.5s** | **0.20** | **26% faster** |
| 15 workers | 95.7s | 0.15 | 0% |
| 5 workers (old default) | 95.8s | 0.15 | 0% |

## Optimization Strategies

### 1. Worker Count Optimization

The number of concurrent workers has the biggest impact on performance:

- **Small workspaces (< 5 pages)**: Use page count as worker count
- **Medium workspaces (5-14 pages)**: Use 20 workers
- **Large workspaces (15+ pages)**: Use 30 workers

### 2. Connection Management

notion-md-sync uses optimized HTTP settings:

- Connection pooling with appropriate limits
- HTTP/2 support for multiplexing
- Proper keep-alive settings
- Optimized timeouts

### 3. Concurrency Model

Simple goroutine-based concurrency with channels:

- Worker pool pattern for controlled concurrency
- Channel-based job distribution
- Graceful error handling per page

## Troubleshooting Performance

### Slow Sync Times

1. **Check worker count**: Ensure you're using optimized settings
   ```yaml
   performance:
     workers: 0  # Let it auto-detect
   ```

2. **Network latency**: The Notion API is the primary bottleneck
   - API calls take 5-7 seconds per page on average
   - Network latency to Notion servers affects performance

3. **Large pages**: Pages with many blocks or databases take longer
   - Consider breaking up very large pages
   - Database exports add additional API calls

### Rate Limiting

If you encounter rate limits:

1. Reduce worker count:
   ```yaml
   performance:
     workers: 15  # Lower from 30
   ```

2. The tool automatically handles rate limiting with retries

### Memory Usage

For very large workspaces:

- Streaming mode is automatically used for 100+ pages
- Memory usage is optimized per worker
- Each worker processes one page at a time

## Best Practices

1. **Use auto-detection**: Set `workers: 0` for optimal performance
2. **Monitor first run**: Check the worker count in output
3. **Adjust if needed**: Fine-tune based on your workspace size
4. **Keep it simple**: Default settings work best for most users

## Technical Details

### Why 30 Workers?

Testing revealed:
- Notion API can handle high concurrency per client
- 30 workers optimally saturate the API without overwhelming it
- Higher counts (50+) showed diminishing returns

### HTTP Client Optimizations

The standard Go HTTP client with default settings performed best:
- Simple is better for API interactions
- Complex optimizations can trigger rate limiting
- Connection reuse is handled automatically

### Future Improvements

Potential areas for further optimization:
- Caching with ETags (not yet supported by Notion API)
- Batch API operations (when Notion adds support)
- Differential sync (only changed blocks)