// Package messagequeue owns Kafka delivery for local development events.
package messagequeue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	mediaapp "github.com/StephenQiu30/then-server/backend/internal/application/media"
	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
)

var channels = map[string]string{
	"then.media-check":            "media.uploaded",
	"then.media-delete":           "media.deletion_requested",
	"then.community-notification": "community.notification_requested",
	"then.generation-wake":        "generation.task_requested",
}

type Broker struct {
	client  *kgo.Client
	brokers []string
	prefix  string
}

type wireEvent struct {
	ID          string `json:"id"`
	EventType   string `json:"event_type"`
	AggregateID string `json:"aggregate_id"`
}

// Open creates only the three declared development topics. Production cluster
// administration and credentials are deliberately outside this local gate.
func Open(ctx context.Context, brokers []string, prefix string) (*Broker, error) {
	if len(brokers) == 0 || len(brokers) > 8 || len(prefix) < 1 || len(prefix) > 80 ||
		strings.Trim(prefix, "abcdefghijklmnopqrstuvwxyz0123456789-") != "" || strings.HasPrefix(prefix, "-") || strings.HasSuffix(prefix, "-") {
		return nil, errors.New("Kafka configuration invalid")
	}
	for _, address := range brokers {
		host, port, err := net.SplitHostPort(address)
		number, parseErr := strconv.Atoi(port)
		ip := net.ParseIP(host)
		if err != nil || parseErr != nil || number < 1 || number > 65535 || (host != "localhost" && (ip == nil || !ip.IsLoopback())) {
			return nil, errors.New("Kafka development requires loopback endpoints")
		}
	}
	client, err := kgo.NewClient(clientOptions(brokers)...)
	if err != nil {
		return nil, errors.New("Kafka configuration invalid")
	}
	b := &Broker{client: client, brokers: append([]string(nil), brokers...), prefix: prefix}
	if err := b.Probe(ctx); err != nil {
		client.Close()
		return nil, err
	}
	request := kmsg.NewPtrCreateTopicsRequest()
	request.TimeoutMillis = 5000
	for channel := range channels {
		topic := kmsg.NewCreateTopicsRequestTopic()
		topic.Topic, topic.NumPartitions, topic.ReplicationFactor = b.topic(channel), 1, 1
		retention, cleanup := "604800000", "delete"
		topic.Configs = []kmsg.CreateTopicsRequestTopicConfig{{Name: "retention.ms", Value: &retention}, {Name: "cleanup.policy", Value: &cleanup}}
		request.Topics = append(request.Topics, topic)
	}
	response, err := request.RequestWith(ctx, client)
	if err != nil {
		client.Close()
		return nil, errors.New("Kafka topic creation unavailable")
	}
	for _, topic := range response.Topics {
		if err := kerr.ErrorForCode(topic.ErrorCode); err != nil && !errors.Is(err, kerr.TopicAlreadyExists) {
			client.Close()
			return nil, errors.New("Kafka topic declaration failed")
		}
	}
	return b, nil
}

func clientOptions(brokers []string) []kgo.Opt {
	// Delivery must outlive a broker request plus metadata discovery and retry.
	// Matching the 10s request timeout caused freshly created topics to fail
	// under the full real-service suite before a retry could finish.
	return []kgo.Opt{kgo.SeedBrokers(brokers...), kgo.ClientID("then-events"),
		kgo.RequiredAcks(kgo.AllISRAcks()), kgo.RecordDeliveryTimeout(30 * time.Second),
		kgo.ProduceRequestTimeout(10 * time.Second),
		kgo.DialTimeout(3 * time.Second), kgo.RequestTimeoutOverhead(3 * time.Second)}
}

func (b *Broker) topic(channel string) string {
	return b.prefix + "." + strings.TrimPrefix(channel, "then.")
}

func (b *Broker) Probe(ctx context.Context) error {
	if b == nil || b.client == nil || b.client.Ping(ctx) != nil {
		return errors.New("Kafka unavailable")
	}
	return nil
}

func (b *Broker) Publish(ctx context.Context, event mediaapp.OutboxEvent) error {
	channel := channelForEvent(event.EventType)
	if b == nil || b.client == nil || channel == "" || event.ID == "" || event.AggregateID == "" {
		return errors.New("message event invalid")
	}
	body, err := json.Marshal(wireEvent{ID: event.ID, EventType: event.EventType, AggregateID: event.AggregateID})
	if err != nil {
		return errors.New("message event invalid")
	}
	if err := b.client.ProduceSync(ctx, &kgo.Record{Topic: b.topic(channel), Key: []byte(event.AggregateID), Value: body, Timestamp: event.CreatedAt}).FirstErr(); err != nil {
		var protocolError *kerr.Error
		if errors.As(err, &protocolError) {
			return fmt.Errorf("Kafka publication not confirmed: %s", protocolError.Message)
		}
		if errors.Is(err, kgo.ErrRecordTimeout) {
			return errors.New("Kafka publication confirmation timed out")
		}
		return errors.New("Kafka publication not confirmed")
	}
	return nil
}

func channelForEvent(eventType string) string {
	for channel, kind := range channels {
		if eventType == kind {
			return channel
		}
	}
	return ""
}

func (b *Broker) Consume(ctx context.Context, channel string, handler func(context.Context, mediaapp.OutboxEvent) error) error {
	kind, valid := channels[channel]
	if b == nil || b.client == nil || !valid || handler == nil {
		return errors.New("message consumer invalid")
	}
	options := append(clientOptions(b.brokers), kgo.ConsumeTopics(b.topic(channel)),
		kgo.ConsumerGroup(b.topic(channel)+".worker"), kgo.DisableAutoCommit(),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()), kgo.BlockRebalanceOnPoll(),
		kgo.RebalanceTimeout(2*time.Minute), kgo.SessionTimeout(10*time.Second))
	consumer, err := kgo.NewClient(options...)
	if err != nil {
		return errors.New("Kafka consumer unavailable")
	}
	defer consumer.CloseAllowingRebalance()
	for {
		fetches := consumer.PollRecords(ctx, 1)
		if ctx.Err() != nil {
			return nil
		}
		if len(fetches.Errors()) > 0 {
			return errors.New("Kafka fetch failed")
		}
		for _, record := range fetches.Records() {
			event, err := decodeRecord(record, kind)
			if err != nil {
				return err
			} // Leave poison records uncommitted for inspection.
			if err := handleRecord(ctx, event, handler); err != nil {
				if ctx.Err() != nil {
					return nil
				}
				return err
			}
			if ctx.Err() != nil {
				return nil
			}
			commit, cancel := context.WithTimeout(ctx, 5*time.Second)
			err = consumer.CommitRecords(commit, record)
			cancel()
			if err != nil {
				if ctx.Err() != nil {
					return nil
				}
				return errors.New("Kafka offset commit failed")
			}
		}
		consumer.AllowRebalance()
	}
}

func decodeRecord(record *kgo.Record, kind string) (mediaapp.OutboxEvent, error) {
	var wire wireEvent
	if record == nil || len(record.Value) > 4096 || json.Unmarshal(record.Value, &wire) != nil ||
		wire.ID == "" || wire.AggregateID == "" || wire.EventType != kind || string(record.Key) != wire.AggregateID {
		return mediaapp.OutboxEvent{}, errors.New("invalid Kafka event; offset retained")
	}
	return mediaapp.OutboxEvent{ID: wire.ID, EventType: wire.EventType, AggregateID: wire.AggregateID, CreatedAt: record.Timestamp}, nil
}

func handleRecord(ctx context.Context, event mediaapp.OutboxEvent, handler func(context.Context, mediaapp.OutboxEvent) error) error {
	for attempt := 0; attempt < 3; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		work, cancel := context.WithTimeout(ctx, 30*time.Second)
		err := handler(work, event)
		if err == nil {
			err = work.Err()
		}
		cancel()
		if err == nil {
			return nil
		}
		if attempt == 2 {
			break
		}
		timer := time.NewTimer(time.Duration(attempt+1) * 250 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return errors.New("Kafka handler retries exhausted; offset retained")
}

func (b *Broker) Close() error {
	if b != nil && b.client != nil {
		b.client.Close()
	}
	return nil
}
