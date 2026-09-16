// Package testcontainer owns lifecycle-safe container fixtures used only by
// external integration and container tests.
//go:build integration || container

package testcontainer

import (
	"context"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
)

// Create registers cleanup for partial creations and successfully started containers.
func Create(t *testing.T, ctx context.Context, request testcontainers.GenericContainerRequest) (testcontainers.Container, error) {
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
