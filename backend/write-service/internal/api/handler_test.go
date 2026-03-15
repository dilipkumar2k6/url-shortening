package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"context"

	"github.com/dilipkumardk/url-shortener/backend/write-service/internal/keygen"
	"github.com/gofiber/fiber/v2"
)

// MockSpannerRepo is a mock implementation of db.SpannerRepo
type MockSpannerRepo struct {
	SaveURLFunc     func(ctx context.Context, id uint64, longURL string) error
	GetURLStaleFunc func(ctx context.Context, shortCode string, staleness time.Duration) (string, error)
}

func (m *MockSpannerRepo) SaveURL(ctx context.Context, id uint64, longURL string) error {
	return m.SaveURLFunc(ctx, id, longURL)
}

func (m *MockSpannerRepo) GetURLStale(ctx context.Context, shortCode string, staleness time.Duration) (string, error) {
	if m.GetURLStaleFunc != nil {
		return m.GetURLStaleFunc(ctx, shortCode, staleness)
	}
	return "", nil
}

func TestHandleShortenRequest(t *testing.T) {
	kg := keygen.NewKeyGenerator(100)

	t.Run("Success", func(t *testing.T) {
		repo := &MockSpannerRepo{
			SaveURLFunc: func(ctx context.Context, id uint64, longURL string) error {
				return nil
			},
		}
		handler := NewHandler(kg, repo, nil, nil)
		app := fiber.New()
		app.Post("/shorten", handler.HandleShortenRequest)

		reqBody := ShortenRequest{LongURL: "https://example.com"}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/shorten", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		resp, _ := app.Test(req)

		if resp.StatusCode != fiber.StatusCreated {
			t.Errorf("Expected status 201, got %d", resp.StatusCode)
		}

		var res ShortenResponse
		json.NewDecoder(resp.Body).Decode(&res)
		if res.ShortURL == "" {
			t.Error("Expected short_url in response")
		}
	})

	t.Run("InvalidBody", func(t *testing.T) {
		repo := &MockSpannerRepo{}
		handler := NewHandler(kg, repo, nil, nil)
		app := fiber.New()
		app.Post("/shorten", handler.HandleShortenRequest)

		req := httptest.NewRequest("POST", "/shorten", bytes.NewBufferString("invalid json"))
		req.Header.Set("Content-Type", "application/json")

		resp, _ := app.Test(req)

		if resp.StatusCode != fiber.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", resp.StatusCode)
		}
	})

	t.Run("MissingLongURL", func(t *testing.T) {
		repo := &MockSpannerRepo{}
		handler := NewHandler(kg, repo, nil, nil)
		app := fiber.New()
		app.Post("/shorten", handler.HandleShortenRequest)

		reqBody := ShortenRequest{LongURL: ""}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/shorten", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		resp, _ := app.Test(req)

		if resp.StatusCode != fiber.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", resp.StatusCode)
		}
	})

	t.Run("SpannerError", func(t *testing.T) {
		repo := &MockSpannerRepo{
			SaveURLFunc: func(ctx context.Context, id uint64, longURL string) error {
				return errors.New("spanner error")
			},
		}
		handler := NewHandler(kg, repo, nil, nil)
		app := fiber.New()
		app.Post("/shorten", handler.HandleShortenRequest)

		reqBody := ShortenRequest{LongURL: "https://example.com"}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/shorten", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		resp, _ := app.Test(req)

		if resp.StatusCode != fiber.StatusInternalServerError {
			t.Errorf("Expected status 500, got %d", resp.StatusCode)
		}
	})
}
