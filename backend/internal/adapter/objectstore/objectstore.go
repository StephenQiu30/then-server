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

	mediaapp "github.com/StephenQiu30/then-server/backend/internal/application/media"

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
