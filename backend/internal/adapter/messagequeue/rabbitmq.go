// Package messagequeue owns durable RabbitMQ delivery for private-media events.
package messagequeue

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	mediaapp "github.com/StephenQiu30/then-server/backend/internal/application/media"

	amqp "github.com/rabbitmq/amqp091-go"
)

const exchangeName = "then.private-media"

type Broker struct {
	connection *amqp.Connection
	publisher  *amqp.Channel
	mu         sync.Mutex
}

type wireEvent struct {
	ID          string `json:"id"`
	EventType   string `json:"event_type"`
	AggregateID string `json:"aggregate_id"`
}

func Open(url string) (*Broker, error) {
	if _, err := amqp.ParseURI(url); err != nil {
		return nil, errors.New("message queue configuration invalid")
	}
	connection, err := amqp.DialConfig(url, amqp.Config{Properties: amqp.Table{"connection_name": "then-private-media"}, Heartbeat: 10 * time.Second, Locale: "en_US"})
	if err != nil {
		return nil, errors.New("message queue unavailable")
	}
	channel, err := connection.Channel()
	if err != nil {
		connection.Close()
		return nil, errors.New("message queue unavailable")
	}
	broker := &Broker{connection: connection, publisher: channel}
	if err := broker.declare(channel); err != nil {
		broker.Close()
		return nil, err
	}
	if err := channel.Confirm(false); err != nil {
		broker.Close()
		return nil, errors.New("message queue publisher confirms unavailable")
	}
	return broker, nil
}

func (b *Broker) declare(channel *amqp.Channel) error {
	if err := channel.ExchangeDeclare(exchangeName, "topic", true, false, false, false, nil); err != nil {
		return errors.New("message queue exchange unavailable")
	}
	for _, binding := range []struct{ queue, key string }{{"then.media-check", "media.uploaded"}, {"then.media-delete", "media.deletion_requested"}} {
		if _, err := channel.QueueDeclare(binding.queue, true, false, false, false, nil); err != nil {
			return errors.New("message queue declaration failed")
		}
		if err := channel.QueueBind(binding.queue, binding.key, exchangeName, false, nil); err != nil {
			return errors.New("message queue binding failed")
		}
	}
	return nil
}

func (b *Broker) Probe(context.Context) error {
	if b == nil || b.connection == nil || b.connection.IsClosed() {
		return errors.New("message queue unavailable")
	}
	return nil
}

func (b *Broker) Publish(ctx context.Context, event mediaapp.OutboxEvent) error {
	if b == nil || b.publisher == nil || event.ID == "" || event.AggregateID == "" || (event.EventType != "media.uploaded" && event.EventType != "media.deletion_requested") {
		return errors.New("message event invalid")
	}
	body, err := json.Marshal(wireEvent{ID: event.ID, EventType: event.EventType, AggregateID: event.AggregateID})
	if err != nil {
		return errors.New("message event invalid")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	confirmation, err := b.publisher.PublishWithDeferredConfirmWithContext(ctx, exchangeName, event.EventType, true, false, amqp.Publishing{ContentType: "application/json", DeliveryMode: amqp.Persistent, MessageId: event.ID, Timestamp: event.CreatedAt, Body: body})
	if err != nil || confirmation == nil {
		return errors.New("message publish failed")
	}
	acknowledged, err := confirmation.WaitContext(ctx)
	if err != nil || !acknowledged {
		return errors.New("message publish not confirmed")
	}
	return nil
}

func (b *Broker) Consume(ctx context.Context, queue string, handler func(context.Context, mediaapp.OutboxEvent) error) error {
	if b == nil || b.connection == nil || handler == nil || (queue != "then.media-check" && queue != "then.media-delete") {
		return errors.New("message consumer invalid")
	}
	channel, err := b.connection.Channel()
	if err != nil {
		return errors.New("message consumer unavailable")
	}
	defer channel.Close()
	if err := channel.Qos(1, 0, false); err != nil {
		return errors.New("message consumer unavailable")
	}
	deliveries, err := channel.Consume(queue, "", false, false, false, false, nil)
	if err != nil {
		return errors.New("message consumer unavailable")
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case delivery, open := <-deliveries:
			if !open {
				return errors.New("message consumer closed")
			}
			var wire wireEvent
			if json.Unmarshal(delivery.Body, &wire) != nil || wire.ID == "" || wire.AggregateID == "" || wire.EventType != delivery.RoutingKey {
				_ = delivery.Nack(false, false)
				continue
			}
			event := mediaapp.OutboxEvent{ID: wire.ID, EventType: wire.EventType, AggregateID: wire.AggregateID, CreatedAt: delivery.Timestamp}
			if err := handler(ctx, event); err != nil {
				_ = delivery.Nack(false, true)
				continue
			}
			if err := delivery.Ack(false); err != nil {
				return errors.New("message acknowledgement failed")
			}
		}
	}
}

func (b *Broker) Close() error {
	if b == nil {
		return nil
	}
	if b.publisher != nil {
		_ = b.publisher.Close()
	}
	if b.connection != nil {
		return b.connection.Close()
	}
	return nil
}
