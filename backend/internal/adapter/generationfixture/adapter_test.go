package generationfixture

import (
	"context"
	"strings"
	"testing"

	generationapp "github.com/StephenQiu30/then-server/backend/internal/application/generation"
)

func TestFixtureProviderCanObserveAcceptedTaskAfterAdapterRestart(t *testing.T) {
	first := &Adapter{image: []byte{1}}
	receipt, err := first.Submit(context.Background(), generationapp.Submission{
		Provider: ProviderName,
		Model:    ImageModel,
		Purpose:  generationapp.PurposeImage,
	})
	if err != nil {
		t.Fatalf("submit fixture task: %v", err)
	}
	if !strings.HasPrefix(receipt.ExternalTaskID, "fixture:") {
		t.Fatalf("fixture task identity has no provider namespace: %q", receipt.ExternalTaskID)
	}

	// The fixture provider has no per-process state, so a new worker instance
	// can safely observe a durable external task identity after restart.
	second := &Adapter{image: []byte{1}}
	remote, err := second.Query(context.Background(), receipt.ExternalTaskID)
	if err != nil || remote.ExternalTaskID != receipt.ExternalTaskID || remote.State != generationapp.StatusSucceeded {
		t.Fatalf("query fixture task after restart: remote=%+v err=%v", remote, err)
	}
	if err := second.Cancel(context.Background(), receipt.ExternalTaskID); err != nil {
		t.Fatalf("cancel fixture task: %v", err)
	}
}

func TestFixtureProviderRejectsOtherProvidersAndPurposesWithoutSideEffects(t *testing.T) {
	adapter := &Adapter{image: []byte{1}}
	for _, submission := range []generationapp.Submission{
		{Provider: "seedream", Model: "remote-image", Purpose: generationapp.PurposeImage},
		{Provider: ProviderName, Model: "fixture-model-v1", Purpose: generationapp.PurposeModel},
	} {
		if _, err := adapter.Submit(context.Background(), submission); err != generationapp.ErrProviderNotAccepted {
			t.Fatalf("unsupported submission was not rejected as known non-acceptance: submission=%+v err=%v", submission, err)
		}
	}
	if _, err := adapter.Query(context.Background(), "remote-task-id"); err == nil {
		t.Fatal("fixture provider accepted a foreign task identity")
	}
}
