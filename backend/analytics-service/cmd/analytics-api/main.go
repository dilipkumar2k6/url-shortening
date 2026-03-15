package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/dilipkumardk/url-shortener/backend/analytics-service/internal"
	"github.com/dilipkumardk/url-shortener/backend/shared/db"
	"github.com/dilipkumardk/url-shortener/backend/shared/telemetry"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"go.opentelemetry.io/otel/log/global"
)

func main() {
	// Ensure Spanner Emulator host is set for the client library
	if host := os.Getenv("SPANNER_EMULATOR_HOST"); host != "" {
		log.Printf("Setting SPANNER_EMULATOR_HOST to %s", host)
		os.Setenv("SPANNER_EMULATOR_HOST", host)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// 1. Setup OpenTelemetry
	shutdown, err := telemetry.SetupOTel(ctx, "analytics-api")
	if err != nil {
		log.Fatalf("Failed to setup OTel: %v", err)
	}
	defer shutdown(context.Background())

	// 2. Initialize ClickHouse
	chAddr := os.Getenv("CLICKHOUSE_ADDR")
	if chAddr == "" {
		chAddr = "clickhouse:9000"
	}
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{chAddr},
		Auth: clickhouse.Auth{
			Database: "default",
			Username: "default",
			Password: "password",
		},
	})
	if err != nil {
		log.Fatalf("Failed to connect to ClickHouse: %v", err)
	}

	// 3. Initialize Spanner
	spannerDB := os.Getenv("SPANNER_DB")
	if spannerDB == "" {
		spannerDB = "projects/url-shortener/instances/main/databases/urls"
	}
	log.Printf("Initializing Spanner Repo with DB: %s", spannerDB)
	repo, err := db.NewSpannerRepo(ctx, spannerDB)
	if err != nil {
		log.Fatalf("Failed to initialize Spanner: %v", err)
	}

	// 4. Setup Fiber App
	app := fiber.New()
	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(telemetry.MetricsMiddleware())
	logger := global.Logger("analytics-api")
	handler := internal.NewHandler(conn, repo, logger)

	app.Get("/api/v1/analytics/top", handler.HandleTopAnalytics)
	app.Get("/api/v1/user/history", handler.HandleUserHistory)

	// Health check
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	// Start server
	go func() {
		log.Println("analytics-api starting on :8080")
		if err := app.Listen(":8080"); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down analytics-api...")
	_ = app.Shutdown()
}
