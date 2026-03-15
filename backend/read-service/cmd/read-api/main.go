package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/dilipkumardk/url-shortener/backend/read-service/internal/api"
	"github.com/dilipkumardk/url-shortener/backend/shared/cache"
	"github.com/dilipkumardk/url-shortener/backend/shared/db"
	"github.com/dilipkumardk/url-shortener/backend/shared/events"
	"github.com/dilipkumardk/url-shortener/backend/shared/telemetry"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func main() {
	// Ensure Spanner Emulator host is set for the client library
	if host := os.Getenv("SPANNER_EMULATOR_HOST"); host != "" {
		os.Setenv("SPANNER_EMULATOR_HOST", host)
	}

	// Initialize Telemetry
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	shutdown, err := telemetry.SetupOTel(ctx, "read-api")
	if err != nil {
		log.Fatalf("failed to initialize OTEL: %v", err)
	}
	defer shutdown(context.Background())

	// Initialize Clients
	spannerDB := os.Getenv("SPANNER_DB")
	if spannerDB == "" {
		spannerDB = "projects/url-shortener/instances/main/databases/urls"
	}
	spannerRepo, err := db.NewSpannerRepo(ctx, spannerDB)
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

	// Initialize Handler
	handler := api.NewReadHandler(spannerRepo, cacheRepo, eventRepo)

	// Setup Fiber
	app := fiber.New(fiber.Config{
		AppName: "URL Shortener Read Service",
	})

	app.Use(logger.New())
	app.Use(recover.New())
	app.Use(telemetry.MetricsMiddleware())

	// Health Check
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	// Redirect Route
	app.Get("/:shortCode", handler.HandleRedirect)

	// Start Server
	go func() {
		if err := app.Listen(":8080"); err != nil {
			log.Fatalf("failed to start server: %v", err)
		}
	}()

	// Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down server...")
	if err := app.Shutdown(); err != nil {
		log.Fatalf("failed to shutdown server: %v", err)
	}
}
