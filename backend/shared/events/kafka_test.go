package events

import (
	"context"
	"testing"
)

func TestEventRepo_EmitCreatedEvent(t *testing.T) {
	repo := NewEventRepo([]string{"localhost:9092"})
	err := repo.EmitCreatedEvent(context.Background(), "abc")
	if err != nil {
		t.Errorf("EmitCreatedEvent failed: %v", err)
	}
}
