package objectstore

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// s3CompatStore implements Store using the AWS SDK v2.
// Works with any S3-compatible API: Cloudflare R2, AWS S3, MinIO, Backblaze B2, etc.
// The only difference between providers is the endpoint URL and region.
type s3CompatStore struct {
	client    *s3.Client
	presigner *s3.PresignClient
	bucket    string
	publicURL string // base URL for public file access, e.g. "https://cdn.example.com"
	provider  string
}

type s3CompatConfig struct {
	accessKey string
	secretKey string
	bucket    string
	publicURL string
	endpoint  string // empty = AWS default; set for R2, MinIO, etc.
	region    string // "auto" for R2, region name for S3
	provider  string // for logging: "r2", "s3"
}

func newS3Compatible(cfg s3CompatConfig) (*s3CompatStore, error) {
	creds := credentials.NewStaticCredentialsProvider(cfg.accessKey, cfg.secretKey, "")

	awsCfg := aws.Config{
		Credentials: creds,
		Region:      cfg.region,
	}

	opts := []func(*s3.Options){
		func(o *s3.Options) {
			// Force path-style so R2 and other non-AWS endpoints work correctly.
			// AWS S3 supports both; R2 requires path-style.
			o.UsePathStyle = true
		},
	}

	if cfg.endpoint != "" {
		opts = append(opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.endpoint)
		})
	}

	client := s3.NewFromConfig(awsCfg, opts...)

	return &s3CompatStore{
		client:    client,
		presigner: s3.NewPresignClient(client),
		bucket:    cfg.bucket,
		publicURL: cfg.publicURL,
		provider:  cfg.provider,
	}, nil
}

func (s *s3CompatStore) Put(ctx context.Context, key string, data []byte, contentType string) (string, error) {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("objectstore %s: put %s: %w", s.provider, key, err)
	}
	return s.publicURL + "/" + key, nil
}

func (s *s3CompatStore) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("objectstore %s: delete %s: %w", s.provider, key, err)
	}
	return nil
}

func (s *s3CompatStore) PresignGet(ctx context.Context, key string, ttl time.Duration, filename string) (string, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}
	if filename != "" {
		// Embed the filename in the signed URL so browsers save with the correct name.
		// Strip quotes to keep the Content-Disposition header well-formed.
		safe := strings.ReplaceAll(filename, `"`, `'`)
		input.ResponseContentDisposition = aws.String(fmt.Sprintf(`attachment; filename="%s"`, safe))
	}
	req, err := s.presigner.PresignGetObject(ctx, input, func(o *s3.PresignOptions) {
		o.Expires = ttl
	})
	if err != nil {
		return "", fmt.Errorf("objectstore %s: presign %s: %w", s.provider, key, err)
	}
	return req.URL, nil
}

func (s *s3CompatStore) Ping(ctx context.Context) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(s.bucket),
	})
	if err != nil {
		return fmt.Errorf("objectstore %s: bucket check failed: %w", s.provider, err)
	}
	return nil
}

func (s *s3CompatStore) Provider() string { return s.provider }
