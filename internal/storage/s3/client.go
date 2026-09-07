package s3

// Package s3 provides the BookDB S3-compatible storage foundation.

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const (
	// MetadataChecksumAlgorithmKey stores the checksum algorithm in user metadata.
	MetadataChecksumAlgorithmKey = "bookdb-checksum-algorithm"
	// MetadataChecksumValueKey stores the checksum value in user metadata.
	MetadataChecksumValueKey = "bookdb-checksum-value"
)

// Checksum describes the checksum metadata attached to an object.
type Checksum struct {
	Algorithm string
	Value     string
}

// PutOptions configures object writes.
type PutOptions struct {
	ContentType string
	Metadata    map[string]string
	Checksum    *Checksum
}

// ObjectInfo captures the stored metadata we care about after a write or read.
type ObjectInfo struct {
	Key         string
	ETag        string
	VersionID   string
	Size        int64
	ContentType string
	Metadata    map[string]string
	Checksum    *Checksum
}

// Store wraps a configured S3-compatible client and default bucket.
type Store struct {
	client *minio.Client
	bucket string
	region string
}

// Options configures a Store.
type Options struct {
	Endpoint       string
	AccessKey      string
	SecretKey      string
	Region         string
	Bucket         string
	ForcePathStyle bool
	Secure         bool
}

// New constructs a store from configuration values.
func New(opts Options) (*Store, error) {
	endpoint, secure, err := normalizeEndpoint(opts.Endpoint, opts.Secure)
	if err != nil {
		return nil, err
	}
	lookup := minio.BucketLookupAuto
	if opts.ForcePathStyle {
		lookup = minio.BucketLookupPath
	}
	client, err := minio.New(endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(opts.AccessKey, opts.SecretKey, ""),
		Region:       opts.Region,
		Secure:       secure,
		BucketLookup: lookup,
	})
	if err != nil {
		return nil, fmt.Errorf("s3: create client: %w", err)
	}
	return &Store{client: client, bucket: opts.Bucket, region: opts.Region}, nil
}

// Check verifies that the bucket exists and the client can reach the service.
func (s *Store) Check(ctx context.Context) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("s3: store is nil")
	}
	if strings.TrimSpace(s.bucket) == "" {
		return fmt.Errorf("s3: bucket is required")
	}
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("s3: bucket probe: %w", err)
	}
	if !exists {
		return fmt.Errorf("s3: bucket %q does not exist", s.bucket)
	}
	return nil
}

// EnsureBucket creates the default bucket when it is missing.
func (s *Store) EnsureBucket(ctx context.Context) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("s3: store is nil")
	}
	if strings.TrimSpace(s.bucket) == "" {
		return fmt.Errorf("s3: bucket is required")
	}
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("s3: bucket probe: %w", err)
	}
	if exists {
		return nil
	}
	if err := s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{Region: s.region}); err != nil {
		return fmt.Errorf("s3: make bucket %q: %w", s.bucket, err)
	}
	return nil
}

// Put stores an object in the configured bucket.
func (s *Store) Put(ctx context.Context, key string, body io.Reader, size int64, opts PutOptions) (ObjectInfo, error) {
	if s == nil || s.client == nil {
		return ObjectInfo{}, fmt.Errorf("s3: store is nil")
	}
	meta := cloneStringMap(opts.Metadata)
	if opts.Checksum != nil {
		meta[MetadataChecksumAlgorithmKey] = opts.Checksum.Algorithm
		meta[MetadataChecksumValueKey] = opts.Checksum.Value
	}
	res, err := s.client.PutObject(ctx, s.bucket, key, body, size, minio.PutObjectOptions{
		ContentType:  opts.ContentType,
		UserMetadata: meta,
	})
	if err != nil {
		return ObjectInfo{}, fmt.Errorf("s3: put object %q: %w", key, err)
	}
	return ObjectInfo{
		Key:         key,
		ETag:        res.ETag,
		VersionID:   res.VersionID,
		Size:        res.Size,
		ContentType: opts.ContentType,
		Metadata:    meta,
		Checksum:    checksumFromMetadata(meta),
	}, nil
}

// Get retrieves an object and its metadata.
func (s *Store) Get(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error) {
	if s == nil || s.client == nil {
		return nil, ObjectInfo{}, fmt.Errorf("s3: store is nil")
	}
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, ObjectInfo{}, fmt.Errorf("s3: get object %q: %w", key, err)
	}
	stat, err := obj.Stat()
	if err != nil {
		_ = obj.Close()
		return nil, ObjectInfo{}, fmt.Errorf("s3: stat object %q: %w", key, err)
	}
	info := objectInfoFromStat(key, stat)
	return obj, info, nil
}

// Delete removes an object from the configured bucket.
func (s *Store) Delete(ctx context.Context, key string) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("s3: store is nil")
	}
	if err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("s3: delete object %q: %w", key, err)
	}
	return nil
}

func objectInfoFromStat(key string, stat minio.ObjectInfo) ObjectInfo {
	return ObjectInfo{
		Key:         key,
		ETag:        stat.ETag,
		VersionID:   stat.VersionID,
		Size:        stat.Size,
		ContentType: stat.ContentType,
		Metadata:    cloneStringMap(stat.UserMetadata),
		Checksum:    checksumFromMetadata(stat.UserMetadata),
	}
}

func checksumFromMetadata(meta map[string]string) *Checksum {
	if len(meta) == 0 {
		return nil
	}
	alg := strings.TrimSpace(meta[MetadataChecksumAlgorithmKey])
	value := strings.TrimSpace(meta[MetadataChecksumValueKey])
	if alg == "" && value == "" {
		return nil
	}
	return &Checksum{Algorithm: alg, Value: value}
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func normalizeEndpoint(raw string, secure bool) (string, bool, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", false, fmt.Errorf("s3: parse endpoint: %w", err)
	}
	if parsed.Host == "" {
		return "", false, fmt.Errorf("s3: endpoint must include host")
	}
	if parsed.Scheme != "" {
		secure = parsed.Scheme == "https"
	}
	return parsed.Host, secure, nil
}
