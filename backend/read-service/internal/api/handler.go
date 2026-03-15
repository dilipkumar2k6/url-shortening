package api

import (
	"context"
	"log"
	"time"

	"github.com/dilipkumardk/url-shortener/backend/shared/cache"
	"github.com/dilipkumardk/url-shortener/backend/shared/db"
	"github.com/dilipkumardk/url-shortener/backend/shared/events"
	"github.com/gofiber/fiber/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/singleflight"
)

type ReadHandler struct {
	db              db.SpannerRepo
	cache           cache.CacheRepo
	events          events.EventRepo
	tracer          trace.Tracer
	logger          otellog.Logger
	sf              singleflight.Group
	errorCounter    metric.Int64Counter
	redirectCounter metric.Int64Counter
	cacheHitCounter metric.Int64Counter
	cacheMissCounter metric.Int64Counter
}

func NewReadHandler(db db.SpannerRepo, cache cache.CacheRepo, events events.EventRepo) *ReadHandler {
	meter := otel.Meter("read-api")
	errorCounter, _ := meter.Int64Counter("redirect_errors_total",
		metric.WithDescription("Total number of errors during URL read/redirect"),
	)
	redirectCounter, _ := meter.Int64Counter("redirect_requests_total",
		metric.WithDescription("Total number of redirect requests"),
	)
	cacheHitCounter, _ := meter.Int64Counter("cache_hits_total",
		metric.WithDescription("Number of successful Redis cache lookups"),
	)
	cacheMissCounter, _ := meter.Int64Counter("cache_misses_total",
		metric.WithDescription("Number of Redis cache misses"),
	)

	return &ReadHandler{
		db:              db,
		cache:           cache,
		events:          events,
		tracer:          otel.Tracer("read-api"),
		logger:          global.Logger("read-api"),
		errorCounter:    errorCounter,
		redirectCounter: redirectCounter,
		cacheHitCounter: cacheHitCounter,
		cacheMissCounter: cacheMissCounter,
	}
}

func (h *ReadHandler) HandleRedirect(c *fiber.Ctx) error {
	shortCode := c.Params("shortCode")
	if len(shortCode) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "short code is required"})
	}

	// OpenTelemetry Logging: Emit a log record for request tracking.
	var record otellog.Record
	record.SetBody(otellog.StringValue("HandleRedirect called"))
	record.SetSeverity(otellog.SeverityInfo)
	record.AddAttributes(otellog.String("short_code", shortCode))
	h.logger.Emit(c.UserContext(), record)

	// Increment redirect_requests_total
	if h.redirectCounter != nil {
		h.redirectCounter.Add(c.UserContext(), 1)
	}

	// OpenTelemetry Tracing: Start a span to track execution time and attributes.
	ctx, span := h.tracer.Start(c.UserContext(), "HandleRedirect", trace.WithAttributes(
		attribute.String("short_code", shortCode),
	))
	defer span.End()

	// Step 1: Bloom Filter Guard
	exists, err := h.cache.CheckBloom(ctx, shortCode)
	if err != nil {
		h.recordError(ctx, span, "bloom_filter_error", err)
	} else if !exists {
		h.recordError(ctx, span, "url_not_found_bloom", nil)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "url not found"})
	}

	// Step 2: Redis Cache Lookup
	longURL, err := h.cache.GetURL(ctx, shortCode)
	if err != nil {
		h.recordError(ctx, span, "redis_get_error", err)
	} else if longURL != "" {
		span.SetAttributes(attribute.String("cache_hit", "redis"))
		if h.cacheHitCounter != nil {
			h.cacheHitCounter.Add(ctx, 1)
		}
		h.emitAnalyticsAsync(shortCode, c)
		c.Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
		return c.Redirect(longURL, fiber.StatusFound)
	}

	if h.cacheMissCounter != nil {
		h.cacheMissCounter.Add(ctx, 1)
	}

	// Step 3: Spanner Stale Read (with Singleflight to prevent cache stampede)
	v, err, _ := h.sf.Do(shortCode, func() (interface{}, error) {
		// Use a detached context with the current span to prevent request cancellation
		// from aborting the shared DB read, while preserving trace propagation.
		sfCtx := trace.ContextWithSpan(context.Background(), trace.SpanFromContext(ctx))
		longURL, err := h.db.GetURLStale(sfCtx, shortCode, 15*time.Second)
		if err != nil {
			return nil, err
		}
		if longURL != "" {
			// Cache warmed inside singleflight
			_ = h.cache.WarmCache(sfCtx, shortCode, longURL)
			_ = h.cache.UpdateBloom(sfCtx, shortCode)
		}
		return longURL, nil
	})

	if err != nil {
		h.recordError(ctx, span, "spanner_read_failed", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "database error"})
	}

	longURL = v.(string)

	if longURL == "" {
		h.recordError(ctx, span, "url_not_found_db", nil)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "url not found"})
	}

	// Tracing Attribute: Record Spanner stale read hit.
	span.SetAttributes(attribute.String("cache_hit", "db_stale"))

	h.emitAnalyticsAsync(shortCode, c)
	c.Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
	return c.Redirect(longURL, fiber.StatusFound)
}

func (h *ReadHandler) emitAnalyticsAsync(shortCode string, c *fiber.Ctx) {
	userAgent := c.Get("User-Agent")
	country := c.Get("CF-IPCountry")
	if country == "" {
		country = "Unknown"
	}

	go func() {
		event := map[string]interface{}{
			"short_code": shortCode,
			"timestamp":  time.Now().Format(time.RFC3339),
			"country":    country,
			"user_agent": userAgent,
		}
		if err := h.events.EmitClickEvent(context.Background(), event); err != nil {
			log.Printf("failed to emit click event to kafka: %v", err)
		}
	}()
}

func (h *ReadHandler) recordError(ctx context.Context, span trace.Span, reason string, err error) {
	if err != nil {
		span.RecordError(err)
	}
	span.SetAttributes(attribute.String("error.reason", reason))
	span.SetStatus(codes.Error, reason)

	if h.errorCounter != nil {
		h.errorCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("reason", reason)))
	}
}
