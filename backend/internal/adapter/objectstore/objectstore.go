// Package objectstore owns the private MinIO adapter for person-photo objects.
package objectstore

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"

	generationapp "github.com/StephenQiu30/then-server/backend/internal/application/generation"
	mediaapp "github.com/StephenQiu30/then-server/backend/internal/application/media"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const (
	RawBucket           = "raw-private"
	DerivedBucket       = "derived-private"
	maxOutputReadURLTTL = 5 * time.Minute
)

type Store struct{ client *minio.Client }

func Open(ctx context.Context, endpoint, accessKey, secretKey string, secure bool) (*Store, error) {
	if endpoint == "" || accessKey == "" || secretKey == "" {
		return nil, errors.New("object store configuration unavailable")
	}
	client, err := minio.New(endpoint, &minio.Options{Creds: credentials.NewStaticV4(accessKey, secretKey, ""), Secure: secure})
	if err != nil {
		return nil, errors.New("object store configuration invalid")
	}
	store := &Store{client: client}
	for _, bucket := range []string{RawBucket, DerivedBucket} {
		exists, err := client.BucketExists(ctx, bucket)
		if err != nil {
			return nil, errors.New("object store unavailable")
		}
		if !exists {
			if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
				return nil, errors.New("private bucket creation failed")
			}
		}
		if err := client.SetBucketVersioning(ctx, bucket, minio.BucketVersioningConfiguration{Status: minio.Enabled}); err != nil {
			return nil, errors.New("private bucket versioning failed")
		}
	}
	return store, nil
}

func (s *Store) Probe(ctx context.Context) error {
	if s == nil || s.client == nil {
		return errors.New("object store unavailable")
	}
	for _, bucket := range []string{RawBucket, DerivedBucket} {
		configuration, err := s.client.GetBucketVersioning(ctx, bucket)
		if err != nil || !configuration.Enabled() {
			return errors.New("object store unavailable")
		}
	}
	return nil
}

func (s *Store) SignUpload(ctx context.Context, asset mediaapp.MediaAsset) (mediaapp.SignedUpload, error) {
	if s == nil || s.client == nil || asset.RawObjectKey == "" || asset.ContentType != mediaapp.MediaContentTypeJPEG || asset.ByteSize <= 0 || asset.SHA256 == "" {
		return mediaapp.SignedUpload{}, errors.New("invalid upload intent")
	}
	headers := http.Header{}
	headers.Set("Content-Type", asset.ContentType)
	headers.Set("Content-Length", strconv.FormatInt(asset.ByteSize, 10))
	headers.Set("X-Amz-Meta-Sha256", asset.SHA256)
	u, err := s.client.PresignHeader(ctx, http.MethodPut, RawBucket, asset.RawObjectKey, mediaapp.UploadIntentLifetime, url.Values{}, headers)
	if err != nil {
		return mediaapp.SignedUpload{}, errors.New("upload signing failed")
	}
	return mediaapp.SignedUpload{Method: http.MethodPut, URL: u.String(), Headers: map[string]string{"Content-Type": asset.ContentType, "Content-Length": strconv.FormatInt(asset.ByteSize, 10), "X-Amz-Meta-Sha256": asset.SHA256}, ExpiresAt: time.Now().UTC().Add(mediaapp.UploadIntentLifetime)}, nil
}

func (s *Store) HeadVersion(ctx context.Context, asset mediaapp.MediaAsset, versionID string) (mediaapp.ObjectFact, error) {
	if s == nil || s.client == nil || asset.RawObjectKey == "" || versionID == "" {
		return mediaapp.ObjectFact{}, errors.New("object fact unavailable")
	}
	info, err := s.client.StatObject(ctx, RawBucket, asset.RawObjectKey, minio.StatObjectOptions{VersionID: versionID})
	if err != nil || info.VersionID != versionID {
		return mediaapp.ObjectFact{}, errors.New("object version unavailable")
	}
	digest := info.Metadata.Get("X-Amz-Meta-Sha256")
	return mediaapp.ObjectFact{ContentType: info.ContentType, ByteSize: info.Size, SHA256: digest}, nil
}

func (s *Store) OpenVersion(ctx context.Context, objectKey, versionID string) (io.ReadCloser, error) {
	return s.openVersion(ctx, RawBucket, objectKey, versionID)
}

func (s *Store) OpenDerivedVersion(ctx context.Context, objectKey, versionID string) (io.ReadCloser, error) {
	return s.openVersion(ctx, DerivedBucket, objectKey, versionID)
}

// ReadOutputVersion returns a bounded generation result from its immutable
// private object version. Callers still verify the content and digest.
func (s *Store) ReadOutputVersion(ctx context.Context, objectKey, versionID string, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		return nil, errors.New("output size limit invalid")
	}
	reader, err := s.OpenDerivedVersion(ctx, objectKey, versionID)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	data, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil || int64(len(data)) > maxBytes {
		return nil, errors.New("output version read failed")
	}
	return data, nil
}

func (s *Store) SignGenerationOutputRead(ctx context.Context, asset generationapp.OutputAsset, ttl time.Duration) (string, time.Time, error) {
	if s == nil || s.client == nil || ttl <= 0 || ttl > maxOutputReadURLTTL || asset.Lineage.OwnerID == "" || asset.Lineage.TaskID == "" || asset.ObjectVersionID == "" {
		return "", time.Time{}, errors.New("generation output access invalid")
	}
	extension := ""
	switch asset.Lineage.Purpose {
	case generationapp.PurposeImage:
		extension = ".jpg"
		if asset.ContentType != generationapp.OutputContentTypeJPEG || asset.ByteSize < 1 || asset.ByteSize > generationapp.MaxGenerationImageOutputBytes {
			return "", time.Time{}, errors.New("generation image output invalid")
		}
	case generationapp.PurposeModel:
		extension = ".glb"
		if asset.ContentType != generationapp.OutputContentTypeGLB || asset.ByteSize < 1 || asset.ByteSize > generationapp.MaxGenerationModelOutputBytes {
			return "", time.Time{}, errors.New("generation model output invalid")
		}
	default:
		return "", time.Time{}, errors.New("generation output purpose invalid")
	}
	expectedKey := "owners/" + asset.Lineage.OwnerID + "/generation/" + asset.Lineage.TaskID + "/output" + extension
	if asset.ObjectKey != expectedKey {
		return "", time.Time{}, errors.New("generation output key invalid")
	}
	parameters := url.Values{"versionId": []string{asset.ObjectVersionID}}
	signedURL, err := s.client.PresignedGetObject(ctx, DerivedBucket, asset.ObjectKey, ttl, parameters)
	if err != nil || signedURL == nil {
		return "", time.Time{}, errors.New("generation output signing failed")
	}
	return signedURL.String(), time.Now().UTC().Add(ttl), nil
}

func (s *Store) openVersion(ctx context.Context, bucket, objectKey, versionID string) (io.ReadCloser, error) {
	if s == nil || s.client == nil || objectKey == "" || versionID == "" {
		return nil, errors.New("object version unavailable")
	}
	object, err := s.client.GetObject(ctx, bucket, objectKey, minio.GetObjectOptions{VersionID: versionID})
	if err != nil {
		return nil, errors.New("object version unavailable")
	}
	if _, err := object.Stat(); err != nil {
		object.Close()
		return nil, errors.New("object version unavailable")
	}
	return object, nil
}

func (s *Store) PutDerived(ctx context.Context, objectKey string, reader io.Reader, size int64) (string, error) {
	if s == nil || s.client == nil || objectKey == "" || reader == nil || size <= 0 {
		return "", errors.New("derived object invalid")
	}
	result, err := s.client.PutObject(ctx, DerivedBucket, objectKey, reader, size, minio.PutObjectOptions{ContentType: mediaapp.MediaContentTypeJPEG})
	if err != nil || result.VersionID == "" {
		return "", errors.New("derived object write failed")
	}
	return result.VersionID, nil
}

func (s *Store) PutArchive(ctx context.Context, objectKey string, reader io.Reader, size int64) (string, error) {
	if s == nil || s.client == nil || objectKey == "" || reader == nil || size <= 0 {
		return "", errors.New("archive invalid")
	}
	result, err := s.client.PutObject(ctx, DerivedBucket, objectKey, reader, size, minio.PutObjectOptions{ContentType: "application/zip"})
	if err != nil || result.VersionID == "" {
		return "", errors.New("archive write failed")
	}
	return result.VersionID, nil
}

func (s *Store) OpenArchive(ctx context.Context, objectKey, versionID string) (io.ReadCloser, error) {
	return s.OpenDerivedVersion(ctx, objectKey, versionID)
}

func (s *Store) DeleteArchive(ctx context.Context, objectKey string) error {
	return s.DeleteAllVersions(ctx, DerivedBucket, objectKey)
}

func (s *Store) DeleteAllVersions(ctx context.Context, bucket, objectKey string) error {
	if s == nil || s.client == nil || (bucket != RawBucket && bucket != DerivedBucket) || objectKey == "" {
		return errors.New("object deletion invalid")
	}
	for object := range s.client.ListObjects(ctx, bucket, minio.ListObjectsOptions{Prefix: objectKey, Recursive: true, WithVersions: true}) {
		if object.Err != nil {
			return errors.New("object version listing failed")
		}
		if object.Key != objectKey {
			continue
		}
		if err := s.client.RemoveObject(ctx, bucket, object.Key, minio.RemoveObjectOptions{VersionID: object.VersionID}); err != nil {
			return errors.New("object version deletion failed")
		}
	}
	return nil
}

// DeleteVersion removes only the object version captured by a cleanup
// manifest. A missing version is already in the desired state; unrelated
// storage failures remain retryable.
func (s *Store) DeleteVersion(ctx context.Context, bucket, objectKey, versionID string) error {
	if s == nil || s.client == nil || (bucket != RawBucket && bucket != DerivedBucket) || objectKey == "" || versionID == "" {
		return errors.New("object version deletion invalid")
	}
	if err := s.client.RemoveObject(ctx, bucket, objectKey, minio.RemoveObjectOptions{VersionID: versionID}); err != nil {
		response := minio.ToErrorResponse(err)
		if response.Code == "NoSuchKey" || response.Code == "NoSuchVersion" {
			return nil
		}
		return errors.New("object version deletion failed")
	}
	return nil
}

// ListOutputVersions returns immutable versions at one exact generation key.
// Prefix neighbors and delete markers are not output data and are excluded.
func (s *Store) ListOutputVersions(ctx context.Context, objectKey string) ([]string, error) {
	if s == nil || s.client == nil || objectKey == "" {
		return nil, errors.New("output version listing unavailable")
	}
	versions := make([]string, 0)
	seen := make(map[string]struct{})
	for object := range s.client.ListObjects(ctx, DerivedBucket, minio.ListObjectsOptions{Prefix: objectKey, Recursive: true, WithVersions: true}) {
		if object.Err != nil {
			return nil, errors.New("output version listing failed")
		}
		if object.Key != objectKey || object.IsDeleteMarker {
			continue
		}
		if object.VersionID == "" {
			return nil, errors.New("output version identity unavailable")
		}
		if _, ok := seen[object.VersionID]; ok {
			continue
		}
		seen[object.VersionID] = struct{}{}
		versions = append(versions, object.VersionID)
	}
	if err := ctx.Err(); err != nil {
		return nil, errors.New("output version listing failed")
	}
	sort.Strings(versions)
	return versions, nil
}
