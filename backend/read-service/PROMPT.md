# Read Service Prompt

Generate a high-performance, stateless Go service for resolving short codes into long URLs and issuing HTTP redirects.

## Core Requirements

1. **Stateless Redirector**:
   - Optimized for high throughput (38,500+ RPS).
   - Strictly read-only; no write operations allowed.
   - Use **Fiber v2** for the HTTP layer.

2. **Multi-Layered Lookup Logic**:
   - **L1: Redis Cache**: Check regional Redis for `url:@short_code`.
   - **L2: Bloom Filter Guard**: Use a Redis-backed Bloom filter to prevent cache penetration. Return 404 if the key definitely doesn't exist.
   - **L3: Spanner Stale Read**: Fallback to Google Cloud Spanner with 15s staleness for sub-10ms consistency.

3. **Resilience (Fail-Open)**:
   - If Redis is unreachable, bypass the cache and query Spanner directly to ensure service availability.

4. **Asynchronous Analytics**:
   - Emit `click-event` to Kafka for every successful redirect.
   - Use a fire-and-forget approach to avoid blocking the redirect response.

5. **Edge Proxy (Envoy)**:
   - Configure Envoy with RE2 regex for short code validation (`^/[a-zA-Z0-9]{7,12}$`).
   - Implement local rate limiting to protect the service from bot traffic.

## Technology Stack
- **Language**: Go 1.24
- **Framework**: Fiber v2
- **Database**: Google Cloud Spanner (Stale Reads)
- **Cache**: Redis (with Bloom filter)
- **Events**: Kafka
- **Proxy**: Envoy
- **Telemetry**: OpenTelemetry & SigNoz
