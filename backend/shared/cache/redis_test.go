package cache

import (
	"context"
	"testing"
)

func TestCacheRepo(t *testing.T) {
	repo := NewCacheRepo("localhost:6379")
	ctx := context.Background()

	t.Run("WarmCache", func(t *testing.T) {
		err := repo.WarmCache(ctx, "abc", "https://example.com")
		if err != nil {
			t.Errorf("WarmCache failed: %v", err)
		}
	})

	t.Run("UpdateBloom", func(t *testing.T) {
		err := repo.UpdateBloom(ctx, "abc")
		if err != nil {
			t.Errorf("UpdateBloom failed: %v", err)
		}
	})
}
