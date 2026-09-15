// Package objectstore owns the private MinIO adapter for person-photo objects.
package objectstore

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/StephenQiu30/then-server/backend/internal/model"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const (
	RawBucket     = "raw-private"
	DerivedBucket = "derived-private"
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

func (s *Store) SignUpload(ctx context.Context, media model.MediaAsset) (model.SignedUpload, error) {
	if s == nil || s.client == nil || media.RawObjectKey == "" || media.ContentType != model.MediaContentTypeJPEG || media.ByteSize <= 0 || media.SHA256 == "" {
		return model.SignedUpload{}, errors.New("invalid upload intent")
	}
	headers := http.Header{}
	headers.Set("Content-Type", media.ContentType)
	headers.Set("Content-Length", strconv.FormatInt(media.ByteSize, 10))
	headers.Set("X-Amz-Meta-Sha256", media.SHA256)
	u, err := s.client.PresignHeader(ctx, http.MethodPut, RawBucket, media.RawObjectKey, model.UploadIntentLifetime, url.Values{}, headers)
	if err != nil {
		return model.SignedUpload{}, errors.New("upload signing failed")
	}
	return model.SignedUpload{Method: http.MethodPut, URL: u.String(), Headers: map[string]string{"Content-Type": media.ContentType, "Content-Length": strconv.FormatInt(media.ByteSize, 10), "X-Amz-Meta-Sha256": media.SHA256}, ExpiresAt: time.Now().UTC().Add(model.UploadIntentLifetime)}, nil
}

func (s *Store) HeadVersion(ctx context.Context, media model.MediaAsset, versionID string) (model.ObjectFact, error) {
	if s == nil || s.client == nil || media.RawObjectKey == "" || versionID == "" {
		return model.ObjectFact{}, errors.New("object fact unavailable")
	}
	info, err := s.client.StatObject(ctx, RawBucket, media.RawObjectKey, minio.StatObjectOptions{VersionID: versionID})
	if err != nil || info.VersionID != versionID {
		return model.ObjectFact{}, errors.New("object version unavailable")
	}
	digest := info.Metadata.Get("X-Amz-Meta-Sha256")
	return model.ObjectFact{ContentType: info.ContentType, ByteSize: info.Size, SHA256: digest}, nil
}

func (s *Store) OpenVersion(ctx context.Context, objectKey, versionID string) (io.ReadCloser, error) {
	if s == nil || s.client == nil || objectKey == "" || versionID == "" {
		return nil, errors.New("object version unavailable")
	}
	object, err := s.client.GetObject(ctx, RawBucket, objectKey, minio.GetObjectOptions{VersionID: versionID})
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
	result, err := s.client.PutObject(ctx, DerivedBucket, objectKey, reader, size, minio.PutObjectOptions{ContentType: model.MediaContentTypeJPEG})
	if err != nil || result.VersionID == "" {
		return "", errors.New("derived object write failed")
	}
	return result.VersionID, nil
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
