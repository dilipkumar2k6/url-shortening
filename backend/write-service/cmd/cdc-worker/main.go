package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/dilipkumardk/url-shortener/backend/shared/cache"
	"github.com/dilipkumardk/url-shortener/backend/shared/db"
	"github.com/dilipkumardk/url-shortener/backend/shared/events"
	"github.com/dilipkumardk/url-shortener/backend/shared/telemetry"
	"github.com/dilipkumardk/url-shortener/backend/write-service/internal/keygen"
	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
)

func main() {
	// Ensure Spanner Emulator host is set for the client library
	if host := os.Getenv("SPANNER_EMULATOR_HOST"); host != "" {
		os.Setenv("SPANNER_EMULATOR_HOST", host)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// 1. Setup OpenTelemetry
	shutdown, err := telemetry.SetupOTel(ctx, "cdc-worker")
	if err != nil {
		log.Fatalf("Failed to setup OTel: %v", err)
	}
	defer shutdown(context.Background())

	// 2. Initialize Dependencies
	spannerDB := os.Getenv("SPANNER_DB")
	if spannerDB == "" {
		spannerDB = "projects/url-shortener/instances/main/databases/urls"
	}
	_, err = db.NewSpannerRepo(ctx, spannerDB)
	if err != nil {
		log.Fatalf("Failed to initialize Spanner: %v", err)
	}

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "redis:6379"
	}
	cacheRepo := cache.NewCacheRepo(redisAddr)

	kafkaBrokers := os.Getenv("KAFKA_BROKERS")
	if kafkaBrokers == "" {
		kafkaBrokers = "kafka:9092"
	}
	eventRepo := events.NewEventRepo([]string{kafkaBrokers})

	log.Println("cdc-worker started, listening for Spanner Change Streams...")

	// 3. Mock Change Stream Processing Loop
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Shutting down cdc-worker...")
			return
		case <-ticker.C:
			// In a real app, we'd read from Spanner Change Stream
			// For now, we'll simulate processing a record
			mockID := uint64(123456)
			mockLongURL := "https://example.com/very/long/url"

			log.Printf("Processing change record: ID=%d, URL=%s", mockID, mockLongURL)
			logger := global.Logger("cdc-worker")
			var record otellog.Record
			record.SetBody(otellog.StringValue("Processing change record"))
			record.SetSeverity(otellog.SeverityInfo)
			record.AddAttributes(
				otellog.Int64("id", int64(mockID)),
				otellog.String("url", mockLongURL),
			)
			logger.Emit(ctx, record)

			// Process record
			processChangeStreamRecord(ctx, mockID, mockLongURL, cacheRepo, eventRepo, logger)
		}
	}
}

func processChangeStreamRecord(ctx context.Context, id uint64, longURL string, cacheRepo cache.CacheRepo, eventRepo events.EventRepo, logger otellog.Logger) {
	// 1. Convert to string for caching layer (Base62)
	shortCode := keygen.Base62Encode(id)

	// 2. Best effort asynchronous updates
	_ = cacheRepo.WarmCache(ctx, shortCode, longURL)
	_ = cacheRepo.UpdateBloom(ctx, shortCode)
	_ = eventRepo.EmitCreatedEvent(ctx, shortCode)

	log.Printf("Successfully processed record for shortCode: %s", shortCode)
}
