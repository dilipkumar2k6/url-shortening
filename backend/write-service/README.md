# Go URL Shortener Write Service

A high-performance URL shortener write service implemented in Go, following a modular architecture and production-ready patterns.

## Architecture

The system is split into two main components to ensure high availability and eventual consistency of the read path. Authentication is handled via **Google Cloud Identity Platform (GCIP) / Firebase Auth**, offloading user management and security to a managed service.

![Component Diagram](images/component_diagram.png)

### Authentication Layer
We use **Firebase Auth** (managed by [console.firebase.google.com](https://console.firebase.google.com)) for several critical reasons:
- **Security**: Offloads sensitive password handling and MFA to Google's secure infrastructure.
- **Ease of Integration**: Provides ready-to-use SDKs for the frontend and standard JWT tokens for the backend.
- **Identity Platform**: Seamlessly scales to Google Cloud Identity Platform (GCIP) for enterprise features like OIDC/SAML.
- **JWT Validation**: Istio Service Mesh validates the JWT tokens at the edge, ensuring only authenticated requests reach the `write-api`.

### Sequence Diagram

![Sequence Diagram](images/sequence_diagram.svg)

### 1. Write Path (`write-api`)
The synchronous path for creating short URLs.
- **Ingestion**: Receives `POST /api/v1/shorten` requests.
- **ID Generation**: Uses a **Distributed Key Generation Service (KGS)** with etcd coordination, dual-buffering, and a Feistel cipher for obfuscation.
- **Persistence**: Saves the mapping `(id, long_url)` to Google Cloud Spanner.
- **Cache Management**: Synchronously warms the Redis cache on creation/update and invalidates on delete to maintain consistency.
- **Response**: Returns a Base62 encoded short code.

### 2. CDC Path (`cdc-worker`)
The asynchronous path for event emission and secondary cache management.
- **Stream Processing**: Listens to Spanner Change Streams.
- **Event Emission**: Emits a `URLCreated` event to Kafka for downstream consumers (e.g., analytics, search indexing).
- **Secondary Cache Management**: Provides eventual consistency and updates the Bloom filter for new insertions.

## API Usage

### Shorten URL
`POST /api/v1/shorten`

**Request:**
```bash
curl -X POST http://localhost:10000/api/v1/shorten \
  -H "Content-Type: application/json" \
  -d '{"long_url": "https://www.google.com"}'
```

**Response:**
```json
{
  "short_url": "http://localhost:10001/abcde123",
  "short_code": "abcde123"
}
```

## Implementation Details

- **Distributed KGS**: Implements a high-performance ID generation service using the **Pre-generated Segment** pattern:
    - **etcd Coordination**: Centralized management of global ID segments (1,000,000 IDs per block).
    - **Dual-Buffering**: Local Primary and Standby buffers with asynchronous prefetching (triggered at 95% usage).
    - **Obfuscation**: 4-round Feistel cipher with **Cycle Walking** to ensure IDs fit within the 7-character Base62 space.
    - **Presentation**: Exactly 7-character Base62 encoded short codes.
- **DB**: Abstracted persistence layer (currently using a Spanner mock).
- **Cache**: Redis integration for fast lookups and Bloom filter for existence checks.
- **Events**: Kafka integration for asynchronous event-driven communication.
- **Telemetry**: Full OpenTelemetry integration with **SigNoz** for unified **Metrics**, **Distributed Traces**, and **Logs**.
- **Rate Limiting**: Managed by Istio Service Mesh.

### Example: Custom Metric in Go
```go
meter := otel.Meter("url-shortener-api")
errorCounter, _ := meter.Int64Counter("shorten_errors_total",
    metric.WithDescription("Total number of errors during URL shortening"),
)
errorCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("reason", "db_error")))
```

## Deployment Strategy

### Docker
Both services are containerized using multi-stage Dockerfiles to ensure minimal image sizes.
- `cmd/write-api/Dockerfile`
- `cmd/cdc-worker/Dockerfile`

### Kubernetes
Manifests are organized into subdirectories for better management:
- `k8s/istio/`: Istio Service Mesh configurations (Gateways, VirtualServices).
- `k8s/write-api/`: Synchronous API service.
- `k8s/cdc-worker/`: Background worker.
- `k8s/infra/`: Local infrastructure (Spanner emulator, etcd, Redis, Kafka).

## Local Development & Testing

I have provided automation scripts to launch the entire setup locally using `kind`.

### Prerequisites
- Docker
- Kind
- Kubectl

### Launch Setup
```bash
./run.sh
```
This script will:
1. Create a `kind` cluster.
2. Build and load Docker images.
3. Deploy all infrastructure and application components.

### Run Verification
```bash
./test.sh
```
This script will:
1. Access the services via the Istio Ingress Gateway.
2. Verify URL shortening functionality.
3. Verify the rate limiter (sending 15 requests to trigger 429).

### Cleanup
```bash
./destroy.sh
```
Deletes the `kind` cluster and all resources.
