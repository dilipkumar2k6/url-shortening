package com.hypershort.analytics;

import org.apache.flink.api.common.eventtime.SerializableTimestampAssigner;
import org.apache.flink.api.common.eventtime.WatermarkStrategy;
import org.apache.flink.api.common.serialization.SimpleStringSchema;
import org.apache.flink.connector.jdbc.JdbcConnectionOptions;
import org.apache.flink.connector.jdbc.JdbcExecutionOptions;
import org.apache.flink.connector.jdbc.JdbcSink;
import org.apache.flink.connector.kafka.source.KafkaSource;
import org.apache.flink.connector.kafka.source.enumerator.initializer.OffsetsInitializer;
import org.apache.flink.shaded.jackson2.com.fasterxml.jackson.databind.JsonNode;
import org.apache.flink.shaded.jackson2.com.fasterxml.jackson.databind.ObjectMapper;
import org.apache.flink.streaming.api.datastream.DataStream;
import org.apache.flink.streaming.api.environment.StreamExecutionEnvironment;
import org.apache.flink.streaming.api.functions.sink.SinkFunction;

import java.time.Duration;
import java.time.LocalDateTime;
import java.time.ZoneOffset;
import java.time.format.DateTimeFormatter;

/**
 * ClickEventsToClickHouse is a Flink DataStream job that consumes URL click events
 * from Kafka, parses the JSON data, and sinks it into ClickHouse via JDBC.
 */
public class ClickEventsToClickHouse {

    public static void main(String[] args) throws Exception {
        // 1. Set up the streaming execution environment
        final StreamExecutionEnvironment env = StreamExecutionEnvironment.getExecutionEnvironment();

        // 2. Load configurations from environment variables (configured via Kubernetes ConfigMap)
        String kafkaBootstrapServers = getRequiredEnv("KAFKA_BOOTSTRAP_SERVERS");
        String clickhouseUrl = getRequiredEnv("CLICKHOUSE_URL");
        String clickhouseUser = getRequiredEnv("CLICKHOUSE_USER");
        String clickhousePassword = getRequiredEnv("CLICKHOUSE_PASSWORD");

        // 3. Configure the Kafka Source
        KafkaSource<String> source = KafkaSource.<String>builder()
                .setBootstrapServers(kafkaBootstrapServers)
                .setTopics("click-events")
                .setGroupId("flink-analytics-java-group")
                .setStartingOffsets(OffsetsInitializer.earliest()) // Start from the beginning of the topic
                .setValueOnlyDeserializer(new SimpleStringSchema()) // Read raw JSON as String
                .build();

        ObjectMapper objectMapper = new ObjectMapper();

        // 4. Define the data processing pipeline
        DataStream<ClickEvent> clickEvents = env.fromSource(source, WatermarkStrategy.noWatermarks(), "Kafka Source")
                // Map: Parse JSON String into ClickEvent POJO
                .map(json -> {
                    JsonNode node = objectMapper.readTree(json);
                    String timestampStr = node.get("timestamp").asText();
                    
                    // Parse RFC3339 string (e.g., 2026-03-14T14:14:54Z) using Instant for better 'Z' handling
                    long epochMilli = java.time.Instant.parse(timestampStr).toEpochMilli();
                    
                    return new ClickEvent(
                            node.get("short_code").asText(),
                            epochMilli,
                            node.get("country").asText()
                    );
                })
                // Strategy: Handle out-of-order events using Watermarks
                .assignTimestampsAndWatermarks(
                        WatermarkStrategy.<ClickEvent>forBoundedOutOfOrderness(Duration.ofSeconds(5)) // Wait up to 5s for late events
                                .withTimestampAssigner((SerializableTimestampAssigner<ClickEvent>) (event, timestamp) -> event.timestamp)
                );

        // 5. Configure the JDBC Sink for ClickHouse
        // We use the MySQL driver as ClickHouse is wire-compatible with MySQL protocol on port 9004
        SinkFunction<ClickEvent> jdbcSink = JdbcSink.sink(
                "INSERT INTO raw_click_events (event_time, short_code, country_code) VALUES (?, ?, ?)",
                (statement, event) -> {
                    statement.setTimestamp(1, new java.sql.Timestamp(event.timestamp));
                    statement.setString(2, event.shortCode);
                    statement.setString(3, event.country);
                },
                JdbcExecutionOptions.builder()
                        .withBatchSize(1000)      // Batch inserts for high throughput
                        .withBatchIntervalMs(200) // Flush every 200ms
                        .withMaxRetries(5)
                        .build(),
                new JdbcConnectionOptions.JdbcConnectionOptionsBuilder()
                        .withUrl(clickhouseUrl)
                        .withDriverName("com.mysql.cj.jdbc.Driver")
                        .withUsername(clickhouseUser)
                        .withPassword(clickhousePassword)
                        .build()
        );

        // 6. Connect the stream to the sink
        clickEvents.addSink(jdbcSink);

        // 7. Trigger the execution
        env.execute("ClickEventsToClickHouseJava");
    }

    private static String getRequiredEnv(String key) {
        String val = System.getenv(key);
        if (val == null || val.isEmpty()) {
            throw new RuntimeException("Environment variable " + key + " is required but missing. Check K8s ConfigMap.");
        }
        return val;
    }

    /**
     * POJO representing a URL Click Event.
     * Needs to be public with a default constructor for Flink serialization.
     */
    public static class ClickEvent {
        public String shortCode;
        public long timestamp;
        public String country;

        public ClickEvent() {}

        public ClickEvent(String shortCode, long timestamp, String country) {
            this.shortCode = shortCode;
            this.timestamp = timestamp;
            this.country = country;
        }
    }
}
