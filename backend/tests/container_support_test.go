//go:build integration || container

package tests

import (
	"context"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
)

// createTestContainer owns partial creations as well as successfully started containers.
func createTestContainer(t *testing.T, ctx context.Context, request testcontainers.GenericContainerRequest) (testcontainers.Container, error) {
	t.Helper()
	container, err := testcontainers.GenericContainer(ctx, request)
	if container != nil {
		t.Cleanup(func() {
			cleanup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := container.Terminate(cleanup); err != nil {
				t.Errorf("owned test container cleanup failed: %v", err)
			}
		})
	}
	return container, err
}
