package api

import (
	"context"
	"log"
	"os"

	"github.com/dilipkumardk/url-shortener/backend/shared/cache"
	"github.com/dilipkumardk/url-shortener/backend/shared/db"
	"github.com/dilipkumardk/url-shortener/backend/shared/events"
	"github.com/dilipkumardk/url-shortener/backend/write-service/internal/keygen"
	"github.com/gofiber/fiber/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

type ShortenRequest struct {
	LongURL    string `json:"long_url"`
	CustomSlug string `json:"custom_slug"`
}

type ShortenResponse struct {
	ShortURL  string `json:"short_url"`
	ShortCode string `json:"short_code"`
}

type Handler struct {
	keyGen         keygen.KeyGenerator
	spannerRepo    db.SpannerRepo
	cacheRepo      cache.CacheRepo
	eventRepo      events.EventRepo
	tracer         trace.Tracer
	logger         otellog.Logger
	errorCounter   metric.Int64Counter
	shortenCounter metric.Int64Counter
}

func NewHandler(kg keygen.KeyGenerator, repo db.SpannerRepo, cache cache.CacheRepo, events events.EventRepo) *Handler {
	tracer := otel.Tracer("url-shortener-api")
	meter := otel.Meter("url-shortener-api")

	// OpenTelemetry Metrics: Define counters for tracking request and error rates.
	shortenCounter, _ := meter.Int64Counter("shorten_requests_total",
		metric.WithDescription("Total number of URL shortening requests"),
	)
	errorCounter, _ := meter.Int64Counter("shorten_errors_total",
		metric.WithDescription("Total number of errors during URL shortening"),
	)

	return &Handler{
		keyGen:         kg,
		spannerRepo:    repo,
		cacheRepo:      cache,
		eventRepo:      events,
		tracer:         tracer,
		logger:         global.Logger("url-shortener-api"),
		errorCounter:   errorCounter,
		shortenCounter: shortenCounter,
	}
}

func (h *Handler) HandleShortenRequest(c *fiber.Ctx) error {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC in HandleShortenRequest: %v", r)
		}
	}()

	log.Printf("HandleShortenRequest called for path: %s", c.Path())
	// OpenTelemetry Logging: Emit a log record for request tracking.
	var record otellog.Record
	record.SetBody(otellog.StringValue("HandleShortenRequest called"))
	record.SetSeverity(otellog.SeverityInfo)
	record.AddAttributes(otellog.String("path", c.Path()))
	h.logger.Emit(c.UserContext(), record)

	// Increment shorten_requests_total counter
	if h.shortenCounter != nil {
		h.shortenCounter.Add(c.UserContext(), 1)
	}

	// OpenTelemetry Tracing: Start a span to track execution time and attributes.
	ctx, span := h.tracer.Start(c.UserContext(), "HandleShortenRequest")
	defer span.End()

	var req ShortenRequest
	if err := c.BodyParser(&req); err != nil {
		h.recordError(ctx, span, "invalid_request_body", err)
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.LongURL == "" {
		h.recordError(ctx, span, "missing_long_url", nil)
		return c.Status(400).JSON(fiber.Map{"error": "long_url is required"})
	}

	span.SetAttributes(attribute.String("long_url", req.LongURL))

	log.Printf("Received shorten request for: %s (custom slug: %s)", req.LongURL, req.CustomSlug)

	userID := c.Get("X-User-Id")
	log.Printf("X-User-Id header: %s", userID)
	var shortCode string

	if userID != "" && req.CustomSlug != "" {
		// Use custom slug if provided by authenticated user
		shortCode = req.CustomSlug
	} else {
		// Generate Base62 slug
		obfuscatedID := h.keyGen.GetNextID()
		shortCode = keygen.Base62Encode(obfuscatedID)
		span.SetAttributes(attribute.Int64("obfuscated_id", int64(obfuscatedID)))
	}

	span.SetAttributes(attribute.String("short_code", shortCode))
	span.SetAttributes(attribute.String("user_id", userID))

	// 2. Synchronous Write to Spanner to ensure consistency and durability
	err := h.spannerRepo.SaveURL(ctx, shortCode, req.LongURL, userID)
	if err != nil {
		log.Printf("Spanner SaveURL failed for %s (slug: %s): %v", req.LongURL, shortCode, err)
		h.recordError(ctx, span, "spanner_insert_failed", err)
		return c.Status(500).JSON(fiber.Map{"error": "Internal server error"})
	}

	// 3. Best-effort asynchronous cache warming and event emission
	// to minimize request latency and decouple from critical path.
	go func() {
		log.Printf("Asynchronously warming cache and emitting events for shortCode: %s", shortCode)
		if err := h.cacheRepo.WarmCache(context.Background(), shortCode, req.LongURL); err != nil {
			log.Printf("Failed to warm cache for %s: %v", shortCode, err)
		}
		if err := h.cacheRepo.UpdateBloom(context.Background(), shortCode); err != nil {
			log.Printf("Failed to update bloom for %s: %v", shortCode, err)
		}
		if err := h.eventRepo.EmitCreatedEvent(context.Background(), shortCode); err != nil {
			log.Printf("Failed to emit created event for %s: %v", shortCode, err)
		}
	}()

	// 4. Return response with full short URL
	baseURL := os.Getenv("SHORT_LINK_BASE_URL")
	if baseURL == "" {
		baseURL = c.Protocol() + "://" + c.Hostname()
	}
	return c.Status(201).JSON(ShortenResponse{
		ShortURL:  baseURL + "/" + shortCode,
		ShortCode: shortCode,
	})
}

func (h *Handler) HandleUpdateURL(c *fiber.Ctx) error {
	userID := c.Get("X-User-Id")
	slug := c.Params("slug")
	log.Printf("HandleUpdateURL called for user: %s, slug: %s", userID, slug)

	if userID == "" {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	var req ShortenRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.LongURL == "" {
		return c.Status(400).JSON(fiber.Map{"error": "long_url is required"})
	}

	ctx, span := h.tracer.Start(c.UserContext(), "HandleUpdateURL", trace.WithAttributes(
		attribute.String("user_id", userID),
		attribute.String("slug", slug),
		attribute.String("long_url", req.LongURL),
	))
	defer span.End()

	err := h.spannerRepo.UpdateURL(ctx, slug, req.LongURL, userID)
	if err != nil {
		log.Printf("Spanner UpdateURL failed for slug %s: %v", slug, err)
		h.recordError(ctx, span, "spanner_update_failed", err)
		return c.Status(500).JSON(fiber.Map{"error": "Internal server error"})
	}

	// Invalidate cache
	if err := h.cacheRepo.DeleteURL(ctx, slug); err != nil {
		log.Printf("Failed to invalidate cache for %s: %v", slug, err)
	}

	// Warm up cache
	if err := h.cacheRepo.WarmCache(ctx, slug, req.LongURL); err != nil {
		log.Printf("Failed to warm cache for %s: %v", slug, err)
	}

	return c.Status(200).JSON(fiber.Map{"message": "URL updated successfully"})
}

func (h *Handler) HandleDeleteURL(c *fiber.Ctx) error {
	userID := c.Get("X-User-Id")
	slug := c.Params("slug")
	log.Printf("HandleDeleteURL called for user: %s, slug: %s", userID, slug)

	if userID == "" {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	ctx, span := h.tracer.Start(c.UserContext(), "HandleDeleteURL", trace.WithAttributes(
		attribute.String("user_id", userID),
		attribute.String("slug", slug),
	))
	defer span.End()

	err := h.spannerRepo.DeleteURL(ctx, slug, userID)
	if err != nil {
		log.Printf("Spanner DeleteURL failed for slug %s: %v", slug, err)
		h.recordError(ctx, span, "spanner_delete_failed", err)
		return c.Status(500).JSON(fiber.Map{"error": "Internal server error"})
	}

	// Invalidate cache
	if err := h.cacheRepo.DeleteURL(ctx, slug); err != nil {
		log.Printf("Failed to invalidate cache for %s: %v", slug, err)
	}

	return c.Status(200).JSON(fiber.Map{"message": "URL deleted successfully"})
}

func (h *Handler) recordError(ctx context.Context, span trace.Span, reason string, err error) {
	if err != nil {
		// Tracing Error: Record error in the span.
		span.RecordError(err)
	}
	// Tracing Attribute: Record error reason and set status.
	span.SetAttributes(attribute.String("error.reason", reason))
	span.SetStatus(codes.Error, reason)
	
	if h.errorCounter != nil {
		// OpenTelemetry Metrics: Increment error counter with reason attribute.
		h.errorCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("reason", reason)))
	}
}
