package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
)

type GCSClient struct {
	client     *storage.Client
	bucketName string
	projectID  string
}

type GCSConfig struct {
	BucketName      string
	ProjectID       string
	CredentialsJSON string
}

func NewGCSClient(ctx context.Context, cfg GCSConfig) (*GCSClient, error) {
	var client *storage.Client
	var err error

	if cfg.CredentialsJSON != "" {
		client, err = storage.NewClient(ctx, option.WithCredentialsJSON([]byte(cfg.CredentialsJSON)))
	} else {
		client, err = storage.NewClient(ctx)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create GCS client: %w", err)
	}

	return &GCSClient{
		client:     client,
		bucketName: cfg.BucketName,
		projectID:  cfg.ProjectID,
	}, nil
}

func (c *GCSClient) Upload(ctx context.Context, key string, data io.Reader, contentType string) (string, error) {
	bucket := c.client.Bucket(c.bucketName)
	obj := bucket.Object(key)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	writer := obj.NewWriter(ctx)
	writer.ContentType = contentType

	if _, err := io.Copy(writer, data); err != nil {
		writer.Close()
		return "", fmt.Errorf("failed to upload to GCS: %w", err)
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("failed to close GCS writer: %w", err)
	}

	return "/" + key, nil
}

func (c *GCSClient) GetURL(key string) string {

	if len(key) > 0 && key[0] == '/' {
		key = key[1:]
	}
	return fmt.Sprintf("https://storage.googleapis.com/%s/%s", c.bucketName, key)
}

func (c *GCSClient) Download(ctx context.Context, key string) ([]byte, error) {

	if len(key) > 0 && key[0] == '/' {
		key = key[1:]
	}

	bucket := c.client.Bucket(c.bucketName)
	obj := bucket.Object(key)

	reader, err := obj.NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to download from GCS: %w", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read GCS object: %w", err)
	}

	return data, nil
}

func (c *GCSClient) Close() error {
	return c.client.Close()
}
