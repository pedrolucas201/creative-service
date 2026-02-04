package config

import (
	"os"
	"time"
)

type Config struct {
	Addr        string
	BaseURL     string
	APIVersion  string
	HTTPTimeout time.Duration

	DatabaseURL string

	S3BucketName string
	S3Region     string
	S3AccessKeyID  string
	S3SecretAccessKey  string

	MaxConcurrency int
}

func Load() Config {
	return Config{
		Addr:        getenv("ADDR", ":8080"),
		BaseURL:     getenv("META_BASE_URL", "https://graph.facebook.com"),
		APIVersion:  getenv("META_API_VERSION", "v24.0"),
		HTTPTimeout: durationDefault(getenv("HTTP_TIMEOUT", "45s"), 45*time.Second),

		DatabaseURL: os.Getenv("DATABASE_URL"),

		S3BucketName: os.Getenv("S3_BUCKET"),
		S3Region:     getenv("S3_REGION", "us-east-1"),
		S3AccessKeyID:  os.Getenv("AWS_ACCESS_KEY_ID"),
		S3SecretAccessKey:  os.Getenv("AWS_SECRET_ACCESS_KEY"),

		MaxConcurrency: atoiDefault(getenv("MAX_CONCURRENCY", "3"), 3),
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
