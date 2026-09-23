package generation

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

// syntheticProvider is a deterministic test double for the asynchronous
// provider port. It never reaches a network and deliberately keeps the same
// idempotency behavior expected from a provider adapter.
type syntheticProvider struct {
	nextID        int
	byIdempotency map[string]Receipt
	states        map[string]RemoteTask
	submissions   []Submission
}

func newSyntheticProvider() *syntheticProvider {
	return &syntheticProvider{
		byIdempotency: make(map[string]Receipt),
		states:        make(map[string]RemoteTask),
	}
}

func (p *syntheticProvider) Submit(ctx context.Context, submission Submission) (Receipt, error) {
	if err := ctx.Err(); err != nil {
		return Receipt{}, err
	}
	if submission.TaskID == "" || submission.Idempotency == "" {
		return Receipt{}, errors.New("invalid synthetic submission")
	}
	if receipt, ok := p.byIdempotency[submission.Idempotency]; ok {
		return receipt, nil
	}
	p.nextID++
	receipt := Receipt{ExternalTaskID: fmt.Sprintf("synthetic-%d", p.nextID)}
	p.byIdempotency[submission.Idempotency] = receipt
	p.states[receipt.ExternalTaskID] = RemoteTask{ExternalTaskID: receipt.ExternalTaskID, State: StatusQueued}
	p.submissions = append(p.submissions, submission)
	return receipt, nil
}

func (p *syntheticProvider) Query(ctx context.Context, externalID string) (RemoteTask, error) {
	if err := ctx.Err(); err != nil {
		return RemoteTask{}, err
	}
	state, ok := p.states[externalID]
	if !ok {
		return RemoteTask{}, errors.New("synthetic task not found")
	}
	return state, nil
}

func (p *syntheticProvider) Cancel(ctx context.Context, externalID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	state, ok := p.states[externalID]
	if !ok {
		return errors.New("synthetic task not found")
	}
	if state.State.terminal() {
		return errors.New("synthetic task already terminal")
	}
	state.State = StatusCanceled
	p.states[externalID] = state
	return nil
}

func (p *syntheticProvider) setState(externalID string, state Status) error {
	remote, ok := p.states[externalID]
	if !ok {
		return errors.New("synthetic task not found")
	}
	remote.State = state
	p.states[externalID] = remote
	return nil
}

func TestSyntheticProviderLifecycleUsesOneSubmissionAndSupportsRecovery(t *testing.T) {
	provider := newSyntheticProvider()
	task, err := NewTask(validCreateInput(), generationTestNow)
	if err != nil {
		t.Fatal(err)
	}
	submission, err := task.BeginSubmission(generationTestNow.Add(time.Minute))
	if err != nil {
		t.Fatalf("PrepareSubmission() error = %v", err)
	}
	if len(submission.Parameters) == 0 || submission.Inputs.LookID != task.LookID || submission.Idempotency != task.IdempotencyKeyHash {
		t.Fatalf("submission lost immutable task facts: %+v", submission)
	}
	submission.Parameters[0] = 'x'
	if task.Parameters[0] == 'x' {
		t.Fatal("submission exposed mutable task parameters")
	}
	receipt, err := provider.Submit(context.Background(), submission)
	if err != nil {
		t.Fatalf("synthetic Submit() error = %v", err)
	}
	if _, err := provider.Submit(context.Background(), submission); err != nil {
		t.Fatalf("idempotent synthetic Submit() error = %v", err)
	}
	if len(provider.submissions) != 1 {
		t.Fatalf("synthetic provider accepted duplicate submissions: %d", len(provider.submissions))
	}
	otherInput := validCreateInput()
	otherInput.ID = "job-2"
	otherInput.OwnerID = "owner-2"
	otherTask, err := NewTask(otherInput, generationTestNow)
	if err != nil {
		t.Fatal(err)
	}
	otherSubmission, err := otherTask.BeginSubmission(generationTestNow.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	otherReceipt, err := provider.Submit(context.Background(), otherSubmission)
	if err != nil {
		t.Fatal(err)
	}
	if otherReceipt.ExternalTaskID == receipt.ExternalTaskID || len(provider.submissions) != 2 {
		t.Fatal("synthetic provider collapsed different owner submissions")
	}
	if err := task.RecordExternalTaskID(receipt.ExternalTaskID, generationTestNow.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := task.PrepareSubmission(); !errors.Is(err, ErrGenerationNotSubmittable) {
		t.Fatalf("accepted task was submitted again: %v", err)
	}
	if err := provider.setState(receipt.ExternalTaskID, StatusRunning); err != nil {
		t.Fatal(err)
	}
	remote, err := provider.Query(context.Background(), receipt.ExternalTaskID)
	if err != nil {
		t.Fatal(err)
	}
	if err := task.ApplyProviderState(remote.ExternalTaskID, remote.State, "", generationTestNow.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := task.RequestCancel(generationTestNow.Add(3 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := provider.Cancel(context.Background(), receipt.ExternalTaskID); err != nil {
		t.Fatal(err)
	}
	remote, err = provider.Query(context.Background(), receipt.ExternalTaskID)
	if err != nil {
		t.Fatal(err)
	}
	if err := task.ApplyProviderState(remote.ExternalTaskID, remote.State, "", generationTestNow.Add(4*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if task.Status != StatusCanceled {
		t.Fatalf("synthetic cancellation did not reconcile: %s", task.Status)
	}
}
