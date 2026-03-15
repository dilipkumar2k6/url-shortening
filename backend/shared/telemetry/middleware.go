package telemetry

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func MetricsMiddleware() fiber.Handler {
	meter := otel.Meter("http-metrics")
	requestsCounter, _ := meter.Int64Counter("http_requests_total",
		metric.WithDescription("Total number of HTTP requests"),
	)
	durationHistogram, _ := meter.Float64Histogram("http_request_duration_seconds",
		metric.WithDescription("Duration of HTTP requests in seconds"),
	)

	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		duration := time.Since(start).Seconds()

		status := c.Response().StatusCode()
		method := c.Method()
		path := "unknown"
		if route := c.Route(); route != nil {
			path = route.Path
		}

		attrs := []attribute.KeyValue{
			attribute.String("method", method),
			attribute.String("path", path),
			attribute.Int("status_code", status),
		}

		requestsCounter.Add(c.UserContext(), 1, metric.WithAttributes(attrs...))
		durationHistogram.Record(c.UserContext(), duration, metric.WithAttributes(attrs...))

		return err
	}
}
