-- Flink SQL script to process click events from Kafka to ClickHouse

CREATE TABLE click_events (
    short_code STRING,
    `timestamp` STRING,
    country STRING,
    -- Parse RFC3339 string (e.g., 2026-03-14T14:14:54Z) to TIMESTAMP(3)
    event_time AS TO_TIMESTAMP(REPLACE(REPLACE(`timestamp`, 'T', ' '), 'Z', ''), 'yyyy-MM-dd HH:mm:ss'),
    WATERMARK FOR event_time AS event_time - INTERVAL '5' SECOND
) WITH (
    'connector' = 'kafka',
    'topic' = 'click-events',
    'properties.bootstrap.servers' = '${KAFKA_BOOTSTRAP_SERVERS}',
    'properties.group.id' = 'flink-analytics-group',
    'scan.startup.mode' = 'earliest-offset',
    'format' = 'json'
);

CREATE TABLE raw_click_events (
    event_time TIMESTAMP(3),
    short_code STRING,
    country_code STRING
) WITH (
    'connector' = 'jdbc',
    'url' = '${CLICKHOUSE_URL}',
    'table-name' = 'raw_click_events',
    'username' = '${CLICKHOUSE_USER}',
    'password' = '${CLICKHOUSE_PASSWORD}',
    'driver' = 'com.mysql.cj.jdbc.Driver'
);

INSERT INTO raw_click_events
SELECT event_time, short_code, country
FROM click_events;
