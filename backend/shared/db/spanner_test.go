package db

import (
	"context"
	"testing"
)

func TestSpannerRepo_SaveURL(t *testing.T) {
	repo, _ := NewSpannerRepo(context.Background(), "projects/p/instances/i/databases/d")
	err := repo.SaveURL(context.Background(), 12345, "https://example.com")
	if err != nil {
		t.Errorf("SaveURL failed: %v", err)
	}
}
