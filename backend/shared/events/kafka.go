package events

import (
	"context"
	"encoding/json"

	"github.com/segmentio/kafka-go"
)

// EventRepo defines the interface for event emission.
type EventRepo interface {
	EmitCreatedEvent(ctx context.Context, shortCode string) error
	EmitClickEvent(ctx context.Context, event map[string]interface{}) error
}

type eventRepo struct {
	brokers []string
}

func NewEventRepo(brokers []string) EventRepo {
	return &eventRepo{brokers: brokers}
}

// EmitCreatedEvent publishes a message to the 'url-created' topic.
func (r *eventRepo) EmitCreatedEvent(ctx context.Context, shortCode string) error {
	w := &kafka.Writer{
		Addr:     kafka.TCP(r.brokers...),
		Topic:    "url-created",
		Balancer: &kafka.LeastBytes{},
	}
	defer w.Close()

	return w.WriteMessages(ctx, kafka.Message{
		Key:   []byte(shortCode),
		Value: []byte(shortCode),
	})
}

// EmitClickEvent publishes a message to the 'click-events' topic.
func (r *eventRepo) EmitClickEvent(ctx context.Context, event map[string]interface{}) error {
	w := &kafka.Writer{
		Addr:     kafka.TCP(r.brokers...),
		Topic:    "click-events",
		Balancer: &kafka.LeastBytes{},
	}
	defer w.Close()

	body, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return w.WriteMessages(ctx, kafka.Message{
		Value: body,
	})
}
