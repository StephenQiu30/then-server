package generation

import (
	"context"
	"errors"
	"testing"
	"time"
)

type timeoutRepositoryStub struct {
	task     Task
	provider string
	cutoff   time.Time
	versions []string
	settled  bool
}

func (r *timeoutRepositoryStub) ListDueTasks(_ context.Context, provider string, cutoff, _ time.Time, _ int) ([]Task, error) {
	r.provider, r.cutoff = provider, cutoff
	return []Task{r.task}, nil
}

func (r *timeoutRepositoryStub) ExpireTaskIfDue(_ context.Context, taskID, provider string, cutoff, _ time.Time, versions []string) (bool, error) {
	if taskID != r.task.ID || provider != r.provider || !cutoff.Equal(r.cutoff) {
		return false, ErrInvalidGenerationInput
	}
	r.versions = append([]string(nil), versions...)
	r.settled = true
	return true, nil
}

type timeoutOutputStub struct {
	key      string
	versions []string
	err      error
}

func (o *timeoutOutputStub) ListOutputVersions(_ context.Context, key string) ([]string, error) {
	o.key = key
	return o.versions, o.err
}

func (o *timeoutOutputStub) ReadOutputVersion(context.Context, string, string, int64) ([]byte, error) {
	return nil, nil
}

func TestTaskTimeoutWorkerInventoriesBeforeSettlement(t *testing.T) {
	task := mustTask(validCreateInput())
	repository := &timeoutRepositoryStub{task: task}
	outputs := &timeoutOutputStub{versions: []string{"version-1"}}
	worker, err := NewTaskTimeoutWorker(repository, outputs, task.Provider, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	worker.now = func() time.Time { return task.CreatedAt.Add(time.Hour) }
	count, err := worker.RunOnce(context.Background())
	key, _ := OutputObjectKey(task)
	if err != nil || count != 1 || !repository.settled || outputs.key != key || len(repository.versions) != 1 || repository.versions[0] != "version-1" || !repository.cutoff.Equal(task.CreatedAt) {
		t.Fatalf("timeout inventory/settlement: count=%d key=%q repository=%+v err=%v", count, outputs.key, repository, err)
	}

	repository.settled = false
	outputs.err = errors.New("store unavailable")
	if _, err := worker.RunOnce(context.Background()); !errors.Is(err, ErrGenerationOutputInventoryUnknown) || repository.settled {
		t.Fatalf("inventory failure settled task: settled=%v err=%v", repository.settled, err)
	}
}
