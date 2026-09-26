package testcontainer

import (
	"os"
	"strings"
)

const defaultMinIOImage = "quay.io/minio/minio:RELEASE.2025-09-07T16-13-09Z@sha256:14cea493d9a34af32f524e538b8346cf79f3321eff8e708c1e2960462bd8936e"

// MinIOImage returns the pinned upstream image by default. CI can supply the
// same pinned release built from source when the upstream registry is unavailable.
func MinIOImage() string {
	if image := strings.TrimSpace(os.Getenv("THEN_TEST_MINIO_IMAGE")); image != "" {
		return image
	}
	return defaultMinIOImage
}
