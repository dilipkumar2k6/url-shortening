package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"

	"github.com/dilipkumardk/url-shortener/backend/shared/cache"
	"github.com/dilipkumardk/url-shortener/backend/shared/db"
	"github.com/dilipkumardk/url-shortener/backend/shared/events"
	"github.com/dilipkumardk/url-shortener/backend/shared/telemetry"
	"github.com/dilipkumardk/url-shortener/backend/write-service/internal/api"
	"github.com/dilipkumardk/url-shortener/backend/write-service/internal/keygen"
	"github.com/gofiber/fiber/v2"
)

func main() {
	// Flag parsing
	uuidGenType := flag.String("uuid-generator", "hlcsnowflake", "Type of UUID generator: dualbuffer or hlcsnowflake")
	flag.Parse()

	// Ensure Spanner Emulator host is set for the client library
	if host := os.Getenv("SPANNER_EMULATOR_HOST"); host != "" {
		os.Setenv("SPANNER_EMULATOR_HOST", host)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// 1. Setup OpenTelemetry
	shutdown, err := telemetry.SetupOTel(ctx, "write-api")
	if err != nil {
		log.Fatalf("Failed to setup OTel: %v", err)
	}
	defer shutdown(context.Background())

	// 2. Initialize Dependencies
	var kg keygen.KeyGenerator
	switch *uuidGenType {
	case "hlcsnowflake":
		log.Println("Using HLC Snowflake generator")
		kg = keygen.NewHLCSnowflakeGenerator()
	case "dualbuffer":
		log.Println("Using Dual Buffer generator")
		etcdEndpoints := os.Getenv("ETCD_ENDPOINTS")
		if etcdEndpoints == "" {
			etcdEndpoints = "etcd:2379"
		}
		kg, err = keygen.NewDualBufferGenerator(strings.Split(etcdEndpoints, ","))
		if err != nil {
			log.Fatalf("Failed to initialize DualBufferGenerator: %v", err)
		}
	default:
		log.Fatalf("Invalid uuid-generator type: %s", *uuidGenType)
	}

	spannerDB := os.Getenv("SPANNER_DB")
	if spannerDB == "" {
		spannerDB = "projects/url-shortener/instances/main/databases/urls"
	}
	repo, err := db.NewSpannerRepo(ctx, spannerDB)
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

	handler := api.NewHandler(kg, repo, cacheRepo, eventRepo)

	// 3. Setup Fiber App
	app := fiber.New()
	app.Use(telemetry.MetricsMiddleware())

	app.Post("/api/v1/shorten", handler.HandleShortenRequest)
	app.Patch("/api/v1/links/:slug", handler.HandleUpdateURL)
	app.Delete("/api/v1/links/:slug", handler.HandleDeleteURL)

	// Health check
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	// Start server
	go func() {
		<-ctx.Done()
		log.Println("Shutting down write-api...")
		_ = app.Shutdown()
	}()

	log.Println("write-api starting on :8080")
	if err := app.Listen(":8080"); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
