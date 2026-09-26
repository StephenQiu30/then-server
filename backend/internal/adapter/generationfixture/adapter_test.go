package generationfixture

import (
	"context"
	"errors"
	"strings"
	"testing"

	generationapp "github.com/StephenQiu30/then-server/backend/internal/application/generation"
	"github.com/google/uuid"
)

type fixtureObjectDeleterStub struct{ targets []generationapp.CleanupTarget }

func (s *fixtureObjectDeleterStub) DeleteObject(_ context.Context, target generationapp.CleanupTarget) error {
	s.targets = append(s.targets, target)
	return nil
}

func TestFixtureCleanupCompletesOnlyStatelessFixtureProviderTasks(t *testing.T) {
	objects := &fixtureObjectDeleterStub{}
	executor, err := NewCleanupExecutor(objects)
	if err != nil {
		t.Fatal(err)
	}
	providerTarget := generationapp.CleanupTarget{Kind: generationapp.CleanupTargetProvider, ID: fixtureTaskID(uuid.NewString())}
	for range 2 {
		if err := executor.DeleteProviderTask(context.Background(), providerTarget); err != nil {
			t.Fatalf("stateless fixture cleanup was not idempotent: %v", err)
		}
	}
	for _, id := range []string{"remote-task", "fixture:not-a-uuid"} {
		err := executor.DeleteProviderTask(context.Background(), generationapp.CleanupTarget{Kind: generationapp.CleanupTargetProvider, ID: id})
		var targetErr *generationapp.CleanupTargetError
		if !errors.As(err, &targetErr) || targetErr.Code != "provider_cleanup_unavailable" {
			t.Fatalf("foreign provider target was treated as fixture: id=%q err=%v", id, err)
		}
	}
	objectTarget := generationapp.CleanupTarget{Kind: generationapp.CleanupTargetObject, ID: uuid.NewString(), ObjectKey: "owners/" + uuid.NewString() + "/generation/" + uuid.NewString() + "/output.jpg", ObjectVersionID: "version"}
	if err := executor.DeleteObject(context.Background(), objectTarget); err != nil || len(objects.targets) != 1 || objects.targets[0] != objectTarget {
		t.Fatalf("fixture cleanup did not delegate exact object deletion: targets=%+v err=%v", objects.targets, err)
	}
}

func TestFixtureProviderCanObserveAcceptedTaskAfterAdapterRestart(t *testing.T) {
	for _, test := range []struct {
		purpose generationapp.Purpose
		model   string
	}{{generationapp.PurposeImage, ImageModel}, {generationapp.PurposeModel, ModelModel}} {
		first := &Adapter{}
		receipt, err := first.Submit(context.Background(), generationapp.Submission{
			Provider: ProviderName,
			Model:    test.model,
			Purpose:  test.purpose,
		})
		if err != nil {
			t.Fatalf("submit fixture task: %v", err)
		}
		if !strings.HasPrefix(receipt.ExternalTaskID, "fixture:") {
			t.Fatalf("fixture task identity has no provider namespace: %q", receipt.ExternalTaskID)
		}
		second := &Adapter{}
		remote, err := second.Query(context.Background(), receipt.ExternalTaskID)
		if err != nil || remote.ExternalTaskID != receipt.ExternalTaskID || remote.State != generationapp.StatusSucceeded {
			t.Fatalf("query fixture task after restart: remote=%+v err=%v", remote, err)
		}
		if err := second.Cancel(context.Background(), receipt.ExternalTaskID); err != nil {
			t.Fatalf("cancel fixture task: %v", err)
		}
	}
}

func TestFixtureProviderRejectsOtherProvidersAndPurposesWithoutSideEffects(t *testing.T) {
	adapter := &Adapter{image: []byte{1}}
	for _, submission := range []generationapp.Submission{
		{Provider: "seedream", Model: "remote-image", Purpose: generationapp.PurposeImage},
		{Provider: ProviderName, Model: ImageModel, Purpose: generationapp.PurposeModel},
		{Provider: ProviderName, Model: ModelModel, Purpose: generationapp.PurposeImage},
	} {
		if _, err := adapter.Submit(context.Background(), submission); err != generationapp.ErrProviderNotAccepted {
			t.Fatalf("unsupported submission was not rejected as known non-acceptance: submission=%+v err=%v", submission, err)
		}
	}
	if _, err := adapter.Query(context.Background(), "remote-task-id"); err == nil {
		t.Fatal("fixture provider accepted a foreign task identity")
	}
}

func TestFixtureGLBPassesModelOutputProfile(t *testing.T) {
	data, err := placeholderGLB()
	if err != nil {
		t.Fatalf("build fixture model: %v", err)
	}
	verified, err := generationapp.VerifyOutputContent(generationapp.PurposeModel, data)
	if err != nil || verified.ContentType != generationapp.OutputContentTypeGLB || verified.ByteSize != int64(len(data)) {
		t.Fatalf("fixture model failed output profile: fact=%+v err=%v", verified, err)
	}
}
