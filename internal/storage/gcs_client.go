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

// GCSConfig contém configurações do GCS
type GCSConfig struct {
	BucketName string
	ProjectID  string
	CredentialsJSON string
}

// NewGCSClient cria um novo cliente GCS
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

// Upload envia um arquivo pro GCS e retorna o path (igual ao S3)
func (c *GCSClient) Upload(ctx context.Context, key string, data io.Reader, contentType string) (string, error) {
	bucket := c.client.Bucket(c.bucketName)
	obj := bucket.Object(key)

	// Timeout de 5 minutos pra upload
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	writer := obj.NewWriter(ctx)
	writer.ContentType = contentType
	// Visibilidade deve ser gerenciada no bucket (IAM/UBLA), não por ACL por objeto.

	if _, err := io.Copy(writer, data); err != nil {
		writer.Close()
		return "", fmt.Errorf("failed to upload to GCS: %w", err)
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("failed to close GCS writer: %w", err)
	}

	// Retorna apenas o path (compatível com sistema atual)
	return "/" + key, nil
}

// GetURL retorna URL pública do arquivo
func (c *GCSClient) GetURL(key string) string {
	// Remove "/" do início se tiver
	if len(key) > 0 && key[0] == '/' {
		key = key[1:]
	}
	return fmt.Sprintf("https://storage.googleapis.com/%s/%s", c.bucketName, key)
}

// Download baixa um arquivo do GCS
func (c *GCSClient) Download(ctx context.Context, key string) ([]byte, error) {
	// Remove "/" do início se tiver
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

// Close fecha o cliente
func (c *GCSClient) Close() error {
	return c.client.Close()
}
