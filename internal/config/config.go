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

	RedisAddr  string
	RedisQueue string

	BlobDir string

	MaxConcurrency int
}

func Load() Config {
	return Config{
		Addr:        getenv("ADDR", ":8080"),
		BaseURL:     getenv("META_BASE_URL", "https://graph.facebook.com"),
		APIVersion:  getenv("META_API_VERSION", "v24.0"),
		HTTPTimeout: durationDefault(getenv("HTTP_TIMEOUT", "45s"), 45*time.Second),

		DatabaseURL: os.Getenv("DATABASE_URL"),

		RedisAddr:  getenv("REDIS_ADDR", "localhost:6379"),
		RedisQueue: getenv("REDIS_QUEUE", "creative_jobs"),

		BlobDir: getenv("BLOB_DIR", "/tmp/blob"),

		MaxConcurrency: atoiDefault(getenv("MAX_CONCURRENCY", "3"), 3),
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
