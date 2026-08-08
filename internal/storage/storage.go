// Package storage wraps the AWS S3 SDK for interacting with Railway's
// native S3-compatible bucket. It stores raw list input, computed results,
// and counter/activity state snapshots under well-known key prefixes, and
// can generate presigned PUT URLs for direct client uploads.
package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Client wraps an S3 client configured for a Railway (or other
// S3-compatible) bucket.
type Client struct {
	s3     *s3.Client
	ps     *s3.PresignClient
	bucket string
}

// Config holds the connection details for the S3-compatible bucket.
type Config struct {
	Endpoint     string
	Bucket       string
	AccessKey    string
	SecretKey    string
	Region       string
	UsePathStyle bool
}

// New creates a Client from the given Config. If Region is empty it
// defaults to "us-east-1".
func New(ctx context.Context, cfg Config) (*Client, error) {
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.Endpoint)
		o.UsePathStyle = cfg.UsePathStyle
	})

	return &Client{
		s3:     s3Client,
		ps:     s3.NewPresignClient(s3Client),
		bucket: cfg.Bucket,
	}, nil
}

func (c *Client) put(ctx context.Context, key string, data []byte) error {
	_, err := c.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &c.bucket,
		Key:    &key,
		Body:   bytes.NewReader(data),
	})
	return err
}

func (c *Client) get(ctx context.Context, key string) ([]byte, error) {
	out, err := c.s3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &c.bucket,
		Key:    &key,
	})
	if err != nil {
		return nil, err
	}
	defer out.Body.Close()
	return io.ReadAll(out.Body)
}

// PutList stores the raw list input for the given job id.
func (c *Client) PutList(ctx context.Context, id string, content []byte) error {
	return c.put(ctx, "lists/"+id, content)
}

// GetList retrieves the raw list input for the given job id.
func (c *Client) GetList(ctx context.Context, id string) ([]byte, error) {
	return c.get(ctx, "lists/"+id)
}

// PutResult stores the result JSON for the given job id.
func (c *Client) PutResult(ctx context.Context, id string, data []byte) error {
	return c.put(ctx, "results/"+id, data)
}

// GetResult retrieves the result JSON for the given job id.
func (c *Client) GetResult(ctx context.Context, id string) ([]byte, error) {
	return c.get(ctx, "results/"+id)
}

// PutState stores a counter/activity snapshot under the given key.
func (c *Client) PutState(ctx context.Context, key string, data []byte) error {
	return c.put(ctx, "state/"+key, data)
}

// GetState retrieves a counter/activity snapshot for the given key.
func (c *Client) GetState(ctx context.Context, key string) ([]byte, error) {
	return c.get(ctx, "state/"+key)
}

// PresignPut generates a presigned URL that allows a client to directly
// PUT the list content for the given job id, valid for ttl.
func (c *Client) PresignPut(ctx context.Context, id string, ttl time.Duration) (string, error) {
	key := "lists/" + id
	req, err := c.ps.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: &c.bucket,
		Key:    &key,
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("presign put: %w", err)
	}
	return req.URL, nil
}
