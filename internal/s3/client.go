package s3

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Client struct {
	s3Client   *s3.Client
	BucketName string
	Region     string
}

type Config struct {
	BucketName      string
	Region          string
	AccessKeyID     string
	SecretAccessKey string
}

func New(ctx context.Context, cfg Config) (*Client, error) {
	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				cfg.AccessKeyID,
				cfg.SecretAccessKey,
				"",
			),
		),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	s3Client := s3.NewFromConfig(awsCfg)

	return &Client{
		s3Client:   s3Client,
		BucketName: cfg.BucketName,
		Region:     cfg.Region,
	}, nil
}

func (c *Client) Upload(ctx context.Context, key string, data io.Reader, contentType string) (string, error) {
	_, err := c.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(c.BucketName),
		Key: aws.String(key),
		Body: data,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("falha ao fazer upload para S3: %w", err)
	}

	url := c.GetURL(key)
	return url, nil
}
	
func (c *Client) GetURL(key string) string {
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", c.BucketName, c.Region, key)
}

func (c *Client) Download(ctx context.Context, key string) ([]byte, error) {
	result, err := c.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.BucketName),
		Key: aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("falha ao baixar do S3: %w", err)
	}
	defer result.Body.Close()
	
	data, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("falha ao ler o corpo do objeto S3: %w", err)
	}

	return data, nil
}