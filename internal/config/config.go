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
	RunMigrations bool

	// AWS S3 (legacy - vai ser removido)
	S3BucketName      string
	S3Region          string
	S3AccessKeyID     string
	S3SecretAccessKey string

	// GCP Cloud Storage (novo)
	GCSBucketName     string
	GCPProjectID      string
	GCSCredentialsJSON string

	// Escolher qual usar: "s3" ou "gcs"
	StorageProvider string

	MaxConcurrency int

	RequireAuth       bool
	FirebaseProjectID string
}

func Load() Config {
	return Config{
		Addr:        getenv("ADDR", ":8080"),
		BaseURL:     getenv("META_BASE_URL", "https://graph.facebook.com"),
		APIVersion:  getenv("META_API_VERSION", "v24.0"),
		HTTPTimeout: durationDefault(getenv("HTTP_TIMEOUT", "45s"), 45*time.Second),

		DatabaseURL: os.Getenv("DATABASE_URL"),
		RunMigrations: boolDefault(getenv("RUN_MIGRATIONS", "false"), false),

		// AWS S3 (legacy)
		S3BucketName:      os.Getenv("S3_BUCKET"),
		S3Region:          getenv("S3_REGION", "us-east-1"),
		S3AccessKeyID:     os.Getenv("AWS_ACCESS_KEY_ID"),
		S3SecretAccessKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),

		// GCP Cloud Storage
		GCSBucketName:      os.Getenv("GCS_BUCKET"),
		GCPProjectID:       os.Getenv("GCP_PROJECT_ID"),
		GCSCredentialsJSON: os.Getenv("GCS_CREDENTIALS_JSON"),

		// Provider: "s3" ou "gcs" (default: s3 pra backward compatibility)
		StorageProvider: getenv("STORAGE_PROVIDER", "s3"),

		MaxConcurrency: atoiDefault(getenv("MAX_CONCURRENCY", "3"), 3),
		RequireAuth:    boolDefault(getenv("AUTH_REQUIRED", "false"), false),
		FirebaseProjectID: getenv("FIREBASE_PROJECT_ID", getenv("GCP_PROJECT_ID", "")),
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
