//go:build services

package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/StephenQiu30/then-server/backend/internal/adapter/messagequeue"
	mediaapp "github.com/StephenQiu30/then-server/backend/internal/application/media"
	"github.com/google/uuid"
	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
)

func kafkaTestPrefix(t *testing.T, ctx context.Context, brokers []string) string {
	t.Helper()
	prefix := "then-test-" + uuid.NewString()
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		client, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
		if err != nil {
			t.Error("Kafka cleanup client failed")
			return
		}
		defer client.Close()
		request := kmsg.NewPtrDeleteTopicsRequest()
		groups := kmsg.NewPtrDeleteGroupsRequest()
		for _, suffix := range []string{"media-check", "media-delete", "community-notification", "generation-wake"} {
			name := prefix + "." + suffix
			request.Topics = append(request.Topics, kmsg.DeleteTopicsRequestTopic{Topic: &name})
			request.TopicNames = append(request.TopicNames, name)
			groups.Groups = append(groups.Groups, name+".worker")
		}
		response, err := request.RequestWith(cleanup, client)
		if err != nil {
			t.Error("Kafka test topic cleanup failed")
			return
		}
		for _, topic := range response.Topics {
			if err := kerr.ErrorForCode(topic.ErrorCode); err != nil && !errors.Is(err, kerr.UnknownTopicOrPartition) {
				t.Error("Kafka test topic deletion rejected")
			}
		}
		result, err := groups.RequestWith(cleanup, client)
		if err != nil {
			t.Error("Kafka test group cleanup failed")
			return
		}
		for _, group := range result.Groups {
			if err := kerr.ErrorForCode(group.ErrorCode); err != nil && !errors.Is(err, kerr.GroupIDNotFound) {
				t.Error("Kafka test group deletion rejected")
			}
		}
	})
	return prefix
}

func TestServicesKafkaRetryRestartAndCommit(t *testing.T) {
	environment := loadServiceEnvironment(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	prefix := kafkaTestPrefix(t, ctx, environment.kafkaBrokers)
	broker, err := messagequeue.Open(ctx, environment.kafkaBrokers, prefix)
	serviceOK(t, "open Kafka broker", err)
	defer broker.Close()
	event := mediaapp.OutboxEvent{ID: uuid.NewString(), AggregateID: uuid.NewString(), EventType: "media.uploaded", CreatedAt: time.Now().UTC()}
	serviceOK(t, "confirmed Kafka publication", broker.Publish(ctx, event))
	calls := 0
	err = broker.Consume(ctx, "then.media-check", func(_ context.Context, got mediaapp.OutboxEvent) error {
		calls++
		if got.ID != event.ID || got.AggregateID != event.AggregateID {
			t.Error("event identity changed")
		}
		return errors.New("synthetic handler failure")
	})
	if err == nil || calls != 3 {
		t.Fatalf("failed record attempts=%d err=%v", calls, err)
	}
	// A separate client verifies progress is in Kafka, not a local cursor.
	serviceOK(t, "close first publisher", broker.Close())
	broker, err = messagequeue.Open(ctx, environment.kafkaBrokers, prefix)
	serviceOK(t, "restart Kafka broker", err)
	defer broker.Close()
	second := event
	second.ID = uuid.NewString()
	serviceOK(t, "publish following record", broker.Publish(ctx, second))
	consume, stop := context.WithCancel(ctx)
	var received []string
	serviceOK(t, "consume after failure", broker.Consume(consume, "then.media-check", func(_ context.Context, got mediaapp.OutboxEvent) error {
		received = append(received, got.ID)
		if got.ID == second.ID {
			stop()
			return context.Canceled
		}
		return nil
	}))
	stop()
	if len(received) != 2 || received[0] != event.ID || received[1] != second.ID {
		t.Fatalf("failed record skipped or reordered: %v", received)
	}
	consume, stop = context.WithCancel(ctx)
	received = nil
	serviceOK(t, "resume committed group", broker.Consume(consume, "then.media-check", func(_ context.Context, got mediaapp.OutboxEvent) error {
		received = append(received, got.ID)
		stop()
		return context.Canceled
	}))
	stop()
	if len(received) != 1 || received[0] != second.ID {
		t.Fatalf("offset did not preserve completed versus cancelled work: %v", received)
	}
}

func TestServicesKafkaInvalidRecordRetainsOffset(t *testing.T) {
	environment := loadServiceEnvironment(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	prefix := kafkaTestPrefix(t, ctx, environment.kafkaBrokers)
	broker, err := messagequeue.Open(ctx, environment.kafkaBrokers, prefix)
	serviceOK(t, "open broker", err)
	defer broker.Close()
	client, err := kgo.NewClient(kgo.SeedBrokers(environment.kafkaBrokers...))
	serviceOK(t, "open invalid record producer", err)
	defer client.Close()
	serviceOK(t, "publish invalid fixture", client.ProduceSync(ctx, &kgo.Record{Topic: prefix + ".media-delete", Value: []byte("invalid")}).FirstErr())
	for range 2 {
		err = broker.Consume(ctx, "then.media-delete", func(context.Context, mediaapp.OutboxEvent) error {
			t.Error("invalid record reached handler")
			return nil
		})
		if err == nil || ctx.Err() != nil {
			t.Fatal("invalid record must fail without advancing its offset")
		}
	}
}

func TestServicesKafkaGenerationOutboxWake(t *testing.T) {
	environment := loadServiceEnvironment(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	prefix := kafkaTestPrefix(t, ctx, environment.kafkaBrokers)
	broker, err := messagequeue.Open(ctx, environment.kafkaBrokers, prefix)
	serviceOK(t, "open Kafka broker", err)
	defer broker.Close()
	event := mediaapp.OutboxEvent{
		ID:          uuid.NewString(),
		AggregateID: uuid.NewString(),
		EventType:   "generation.task_requested",
		CreatedAt:   time.Now().UTC(),
	}
	serviceOK(t, "publish generation wake", broker.Publish(ctx, event))

	consume, stop := context.WithCancel(ctx)
	defer stop()
	serviceOK(t, "consume generation wake", broker.Consume(consume, "then.generation-wake", func(_ context.Context, got mediaapp.OutboxEvent) error {
		if got.ID != event.ID || got.AggregateID != event.AggregateID || got.EventType != event.EventType {
			t.Errorf("generation wake identity changed: got=%+v want=%+v", got, event)
		}
		stop()
		return context.Canceled
	}))
}
