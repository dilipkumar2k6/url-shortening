DROP TABLE IF EXISTS click_stats_mv;
DROP TABLE IF EXISTS click_stats_hourly;
DROP TABLE IF EXISTS raw_click_events;

CREATE TABLE raw_click_events (
    event_time DateTime,
    short_code String,
    country_code String
) ENGINE = Null;

CREATE TABLE click_stats_hourly (
    hour DateTime,
    short_code String,
    country_code String,
    total_clicks AggregateFunction(count, UInt64)
) ENGINE = SummingMergeTree()
ORDER BY (hour, short_code, country_code);

CREATE MATERIALIZED VIEW click_stats_mv TO click_stats_hourly AS
SELECT
    toStartOfHour(event_time) AS hour,
    short_code,
    country_code,
    countState() AS total_clicks
FROM raw_click_events
GROUP BY hour, short_code, country_code;
