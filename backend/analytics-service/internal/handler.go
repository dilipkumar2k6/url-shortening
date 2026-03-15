package internal

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/dilipkumardk/url-shortener/backend/shared/db"
	"github.com/gofiber/fiber/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

type Handler struct {
	chConn       clickhouse.Conn
	dbRepo       db.SpannerRepo
	logger       otellog.Logger
	tracer       trace.Tracer
	errorCounter metric.Int64Counter
}

func NewHandler(ch clickhouse.Conn, repo db.SpannerRepo, logger otellog.Logger) *Handler {
	tracer := otel.Tracer("analytics-api")
	meter := otel.Meter("analytics-api")
	errorCounter, _ := meter.Int64Counter("analytics_errors_total",
		metric.WithDescription("Total number of errors during analytics operations"),
	)

	return &Handler{
		chConn:       ch,
		dbRepo:       repo,
		logger:       logger,
		tracer:       tracer,
		errorCounter: errorCounter,
	}
}

func (h *Handler) HandleTopAnalytics(c *fiber.Ctx) error {
	// OpenTelemetry Tracing: Start a span to track execution time.
	ctx, span := h.tracer.Start(c.UserContext(), "HandleTopAnalytics")
	defer span.End()

	// OpenTelemetry Logging: Emit a log record for request tracking.
	var record otellog.Record
	record.SetBody(otellog.StringValue("Top links analytics requested"))
	record.SetSeverity(otellog.SeverityInfo)
	h.logger.Emit(ctx, record)

	var stats []TopLinkStats
	// Query ClickHouse for top 10 short codes by total clicks
	// using aggregated hourly stats for efficiency.
	query := `
		SELECT short_code, countMerge(total_clicks) as clicks
		FROM click_stats_hourly
		GROUP BY short_code
		ORDER BY clicks DESC
		LIMIT 10
	`
	if err := h.chConn.Select(ctx, &stats, query); err != nil {
		h.recordError(ctx, span, "clickhouse_query_failed", err)
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// Fetch LongURL from Spanner for each short_code to enrich the response.
	// We use stale reads (15s) for better performance, falling back to
	// strong reads (staleness = 0) if the stale read fails.
	var enrichedStats []TopLinkStats
	for _, stat := range stats {
		longURL, err := h.dbRepo.GetURLStale(ctx, stat.ShortCode, 15*time.Second)
		if err != nil {
			span.AddEvent("stale_read_failed", trace.WithAttributes(attribute.String("short_code", stat.ShortCode)))
			longURL, err = h.dbRepo.GetURLStale(ctx, stat.ShortCode, 0)
			if err != nil {
				span.AddEvent("strong_read_failed", trace.WithAttributes(attribute.String("short_code", stat.ShortCode)))
				continue
			}
		}
		// Filter out links that no longer exist in Spanner (e.g., deleted)
		if longURL != "" {
			stat.LongURL = longURL
			enrichedStats = append(enrichedStats, stat)
		}
	}

	return c.JSON(TopLinksResponse{Links: enrichedStats})
}

func (h *Handler) HandleUserHistory(c *fiber.Ctx) error {
	userID := c.Get("X-User-Id")
	if userID == "" {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	ctx, span := h.tracer.Start(c.UserContext(), "HandleUserHistory", trace.WithAttributes(
		attribute.String("user_id", userID),
	))
	defer span.End()

	history, err := h.dbRepo.GetUserHistory(ctx, userID)
	if err != nil {
		h.recordError(ctx, span, "spanner_history_failed", err)
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(history)
}

func (h *Handler) recordError(ctx context.Context, span trace.Span, reason string, err error) {
	if err != nil {
		span.RecordError(err)
	}
	span.SetAttributes(attribute.String("error.reason", reason))
	span.SetStatus(codes.Error, reason)

	if h.errorCounter != nil {
		h.errorCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("reason", reason)))
	}
}
