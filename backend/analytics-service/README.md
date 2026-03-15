# URL Shortener Analytics Service

The Analytics Service is responsible for processing click events and providing real-time insights into URL performance.

## Features

- **Real-Time Ingestion**: Processes click events from Kafka using **Apache Flink**.
- **High-Performance Analytics**: Leverages ClickHouse for fast aggregation and querying.
- **Data Enrichment**: Enriches analytics data with URL metadata from Spanner, filtering out deleted links.
- **Observability**: Fully instrumented with OpenTelemetry for monitoring and tracing.

## Architecture

The service consists of an **Apache Flink** cluster for stream processing and a Go-based API for data retrieval.

![Component Diagram](images/component_diagram.png)

### Sequence Diagram

![Sequence Diagram](images/sequence_diagram.svg)

## API Usage

### Get Top Performing Links
`GET /api/v1/analytics/top`

**Request:**
```bash
curl -i http://localhost:10000/api/v1/analytics/top?page=1&limit=10
```

**Response:**
```json
{
  "top_links": [
    {
      "short_code": "abcde123",
      "long_url": "https://www.google.com",
      "total_clicks": 150
    }
  ]
}
```

## Implementation Details

- **Apache Flink**: Consumes click events from Kafka and writes them to ClickHouse using Flink SQL.
- **ClickHouse**: Uses Materialized Views to aggregate raw events into hourly statistics.
- **Analytics API**: Queries ClickHouse for top links and fetches `long_url` from Spanner using stale reads, filtering out entries for links that have been deleted.
- **OpenTelemetry**: Exports metrics, traces, and logs to SigNoz for comprehensive observability.

## Deployment Strategy

### Docker
The service components are containerized:
- `cmd/analytics-api/Dockerfile`
- `flink/Dockerfile` (Custom Flink image with Kafka/JDBC connectors)

### Kubernetes
Manifests are organized into subdirectories:
- `k8s/analytics-api/`: API service for analytics retrieval.
- `k8s/flink/`: Apache Flink JobManager and TaskManager.

## Local Development & Testing

### Launch Setup
```bash
./run.sh
```

### Run Verification
```bash
./test.sh
```

### Cleanup
```bash
./destroy.sh
```
