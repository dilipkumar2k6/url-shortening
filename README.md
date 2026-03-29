# HyperShort: High-Performance URL Shortener

HyperShort is a production-ready URL shortener system built with a modern stack, focusing on high performance, scalability, and observability.

## System Architecture

The system is designed with a clear separation between write and read paths to optimize for both high-throughput shortening and low-latency redirects.

## Architecture

![System Architecture](images/system_architecture.png)

### Sequence Diagram

![Sequence Diagram](images/sequence_diagram.svg)

## Istio Service Mesh Architecture

The system uses **Istio Service Mesh** to manage traffic, security, and observability across all microservices, replacing the previous manual Envoy configuration.

### Why Istio?
1. **Traffic Isolation**: High-volume redirect traffic (clicks) is handled efficiently and isolated from management traffic using localized routing rules.
2. **Security**: Handles JWT validation and mTLS transparently at the platform level.
3. **Performance**: Optimized routing with low overhead for high-speed redirects.

### Rate Limiting Strategy
The system continues to employ a defense-in-depth rate limiting strategy:
1.  **Local Rate Limiting**: Enforced by Istio's sidecar proxies or gateways.
2.  **Global Rate Limiting**: Still supported via integration with external rate limiting services if needed, or managed via Istio's advanced traffic management features.


## Core Components

### 1. Write Service (`write-api`)
Handles the synchronous path for creating short URLs.
- **ID Generation**: Uses a Feistel cipher to obfuscate sequential IDs, preventing short URL guessing.
- **Persistence**: Saves the mapping `(id, long_url)` to Google Cloud Spanner.
- **Cache Management**: Synchronously warms the Redis cache on creation/update and invalidates on delete to maintain consistency.
- **Async Tasks**: Emits a `url-created` event to Kafka for downstream processing.

### 2. Read Service (`read-api`)
Optimized for low-latency redirects.
- **Multi-Layer Lookup**: Checks Bloom Filter guard first, then Redis cache, and falls back to Spanner for stale reads.
- **Cache Stampede Prevention**: Uses `singleflight` to collapse concurrent requests for the same short code during cache misses.
- **Dynamic Updates**: Uses `302 Found` redirects with `Cache-Control: no-store` to prevent browser caching, ensuring edits take effect immediately.
- **Resilience**: Implements a "Fail-Open" strategy to ensure redirects work even if non-critical components (Redis, Bloom) are down.
- **Analytics**: Emits `click-event` to Kafka for every successful redirect.

### 3. Analytics Service
Processes and serves real-time link performance data.
- **Stream Processing**: Uses **Apache Flink** to consume `click-event` from Kafka and sink them into ClickHouse with exactly-once semantics.
- **OLAP Storage**: Leverages **ClickHouse** for high-performance aggregations and Materialized Views.
- **API**: Provides high-performance querying of ClickHouse for the top-performing links dashboard, enriched with metadata from Spanner.

### 4. Frontend
A modern React application built with **Tailwind CSS** and **Shadcn UI**.
- **Real-time Updates**: Uses `react-query` for efficient data fetching and caching (5s refetch interval).
- **Responsive Design**: Fully responsive UI for shortening URLs and viewing analytics.

## Key Features

### Dynamic Domain Support
The system supports a dynamic base URL for shortened links, configured via the `SHORT_LINK_BASE_URL` environment variable. This allows the system to generate links that match the environment (e.g., `localhost:10001` for local dev, `sho.rt` for production).

## Observability (OpenTelemetry & SigNoz)

The system is fully instrumented with **OpenTelemetry** for distributed tracing, metrics, and logging.

> [!NOTE]
> **SigNoz Infrastructure**: SigNoz runs with a single-node ClickHouse cluster and a dedicated **ZooKeeper** instance to support schema migrations and distributed operations.

### Metrics Covered
We track several key performance indicators (KPIs) across the system:

| Metric Name | Description | Type |
|-------------|-------------|------|
| `shorten_requests_total` | Total number of URL shortening requests | Counter |
| `shorten_errors_total` | Total number of errors during shortening | Counter |
| `redirect_requests_total` | Total number of redirect requests | Counter |
| `cache_hits_total` | Number of successful Redis cache lookups | Counter |
| `cache_misses_total` | Number of Redis cache misses | Counter |



### Accessing SigNoz UI

SigNoz provides a unified dashboard for all telemetry data (Traces, Metrics, Logs).

#### 1. Verify SigNoz is Running
Before accessing the UI, ensure all SigNoz components are ready:
```bash
kubectl get pods -l 'app in (signoz-frontend, signoz-query-service, signoz-clickhouse, signoz-otel-collector, signoz-zookeeper)'
```
*All pods should be in `Running` state.*

#### 2. Port-Forward Frontend
`run.sh` automatically port-forwards the SigNoz frontend. If you need to do it manually:
```bash
kubectl port-forward svc/signoz-frontend 3301:3301
```

#### 3. Access the Dashboard
Open your browser and navigate to:
[http://localhost:3301](http://localhost:3301)

> [!NOTE]
> **First-time Setup**: On your first visit, SigNoz will prompt you to create an admin account.

#### Resetting Admin Account

If you are prompted for an invitation or need to reset the admin account, you can restart the `signoz-query-service` pod to clear the ephemeral database:

```bash
export KUBECONFIG=$HOME/.kube/kind-url-shortener
kubectl rollout restart deployment/signoz-query-service
```
This will allow you to create a new admin account on the next visit.

## Getting Started

1. **Launch Cluster**: `./run.sh` (Creates Kind cluster and deploys all services)
2. **Verify**: `./test.sh` (Runs integration tests)
3. **Cleanup**: `./destroy.sh` (Deletes cluster)
