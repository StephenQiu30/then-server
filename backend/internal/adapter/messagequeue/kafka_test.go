package messagequeue

import (
	"context"
	"errors"
	"testing"

	mediaapp "github.com/StephenQiu30/then-server/backend/internal/application/media"
	"github.com/twmb/franz-go/pkg/kgo"
)

func TestRecordValidationRejectsWrongRouteAndIdentity(t *testing.T) {
	for _, test := range []struct {
		name, key, body string
		valid           bool
	}{
		{"valid", "asset", `{"id":"event","aggregate_id":"asset","event_type":"media.uploaded"}`, true},
		{"wrong channel", "asset", `{"id":"event","aggregate_id":"asset","event_type":"media.deletion_requested"}`, false},
		{"wrong key", "other", `{"id":"event","aggregate_id":"asset","event_type":"media.uploaded"}`, false},
		{"missing identity", "asset", `{"aggregate_id":"asset","event_type":"media.uploaded"}`, false},
		{"malformed", "asset", `not-json`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := decodeRecord(&kgo.Record{Key: []byte(test.key), Value: []byte(test.body)}, "media.uploaded")
			if (err == nil) != test.valid {
				t.Fatalf("valid=%v error=%v", test.valid, err)
			}
		})
	}
}

func TestHandlerRetriesAndCancellation(t *testing.T) {
	calls := 0
	if err := handleRecord(context.Background(), mediaapp.OutboxEvent{}, func(context.Context, mediaapp.OutboxEvent) error {
		calls++
		if calls == 1 {
			return errors.New("transient")
		}
		return nil
	}); err != nil || calls != 2 {
		t.Fatalf("retry recovery calls=%d err=%v", calls, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	calls = 0
	err := handleRecord(ctx, mediaapp.OutboxEvent{}, func(context.Context, mediaapp.OutboxEvent) error { calls++; cancel(); return nil })
	if !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatal("cancelled work must not be reported complete")
	}
}

func TestOpenRejectsUnsafeConfigurationBeforeDial(t *testing.T) {
	for _, test := range []struct{ address, prefix string }{
		{"example.com:9092", "then"}, {"127.0.0.1:0", "then"}, {"127.0.0.1:9092", "../other"}, {"127.0.0.1:9092", ""},
	} {
		if broker, err := Open(context.Background(), []string{test.address}, test.prefix); err == nil || broker != nil {
			t.Fatal("unsafe Kafka configuration accepted")
		}
	}
}
