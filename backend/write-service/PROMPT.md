# Write Service Prompt

Generate a high-performance URL shortener write service with CDC-based cache warming and event emission.

## Core Requirements

1. **Synchronous HTTP API (`write-api`)**:
   - Use **Fiber v2** for the HTTP layer.
   - Implement `POST /api/v1/shorten`.
   - **ID Generation**: Use a **Distributed Key Generation Service (KGS)** with the **Pre-generated Segment** pattern:
     - **etcd Coordination**: Manage global ID segments (1,000,000 IDs per block).
     - **Dual-Buffering**: Implement local Primary and Standby buffers with asynchronous prefetching (triggered at 95% usage).
     - **Obfuscation**: Use a **Feistel cipher** with **Cycle Walking** to ensure IDs fit within the 7-character Base62 space.
   - **Encoding**: Use **Base62** for the presentation layer, ensuring exactly **7-character** short codes.
   - **Persistence**: Save the mapping `(id, long_url)` to **Google Cloud Spanner**.
   - **Dynamic Domain**: Support a configurable base URL via `SHORT_LINK_BASE_URL`.

2. **Background CDC Processor (`cdc-worker`)**:
   - Listen to **Spanner Change Streams**.
   - **Cache Warming**: For every new insertion, populate **Redis** and update the **Bloom filter**.
   - **Event Emission**: Emit a `url-created` event to **Kafka** for downstream analytics.

3. **Edge Proxy (Envoy)**:
   - Configure Envoy with local rate limiting (10 RPS) to protect the write path.

4. **Authentication (GCIP / Firebase Auth)**:
   - Integrate with **Firebase Auth** for user management.
   - Use **Envoy JWT Authentication** filter to validate tokens at the edge.
   - Extract `sub` claim from JWT and pass it as `X-User-Id` header to the backend.

5. **Observability**:
   - Full instrumentation with **OpenTelemetry** (Metrics, Traces, Logs).
   - Export telemetry to **SigNoz**.

## Technology Stack
- **Language**: Go 1.24
- **Framework**: Fiber v2
- **Authentication**: Firebase Auth / GCIP
- **Database**: Google Cloud Spanner
- **Cache**: Redis (with Bloom filter)
- **Coordination**: etcd (for KGS)
- **Events**: Kafka
- **Proxy**: Envoy
- **Telemetry**: OpenTelemetry & SigNoz
