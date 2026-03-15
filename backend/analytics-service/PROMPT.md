# Analytics Service Prompt

Generate a high-performance real-time analytics service for tracking URL performance.

## Core Requirements

1. **Stream Processing (Apache Flink)**:
   - Consume `click-events` from Kafka.
   - Parse RFC3339 timestamps and extract `short_code` and `country_code`.
   - Sink data into **ClickHouse** using the JDBC connector.
   - Use **Flink SQL** for the processing pipeline to ensure scalability and maintainability.

2. **OLAP Storage (ClickHouse)**:
   - Use a `Null` engine table for raw event ingestion.
   - Implement a **Materialized View** to aggregate clicks into hourly buckets (`SummingMergeTree`).
   - Ensure the schema supports high-concurrency analytical queries.

3. **Analytics API (Go/Fiber)**:
   - Implement `GET /api/v1/analytics/top` to fetch the top 10 performing links.
   - Query ClickHouse for aggregated click counts.
   - **Data Enrichment**: For each `short_code`, fetch the corresponding `long_url` from **Google Cloud Spanner** using stale reads (15s) for maximum performance.

4. **Observability**:
   - Full instrumentation with **OpenTelemetry** (Metrics, Traces, Logs).
   - Export telemetry to **SigNoz**.

## Technology Stack
- **Language**: Go 1.24
- **Framework**: Fiber v2
- **Stream Processing**: Apache Flink 1.18
- **Databases**: ClickHouse, Google Cloud Spanner
- **Messaging**: Kafka
- **Telemetry**: OpenTelemetry & SigNoz
