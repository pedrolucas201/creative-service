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

	DatabaseURL   string
	RunMigrations bool

	S3BucketName      string
	S3Region          string
	S3AccessKeyID     string
	S3SecretAccessKey string

	GCSBucketName      string
	GCPProjectID       string
	GCSCredentialsJSON string

	StorageProvider string

	MaxConcurrency int

	RequireAuth            bool
	FirebaseProjectID      string
	MetaWebhookVerifyToken string
	MetaWebhookAppSecret   string

	ContingencyInternalToken        string
	ContingencyMonitorAdAccounts    []string
	ContingencyDefaultMaxCandidates int
	ContingencyDefaultMaxAttempts   int
	ContingencyDefaultRefreshStatus bool
	ContingencyDispatchViaTasks     bool
	ContingencyTasksProjectID       string
	ContingencyTasksLocation        string
	ContingencyTasksQueue           string
	ContingencyTasksExecuteURL      string
}

func Load() Config {
	return Config{
		Addr:        getenv("ADDR", ":8080"),
		BaseURL:     getenv("META_BASE_URL", "https://graph.facebook.com"),
		APIVersion:  getenv("META_API_VERSION", "v24.0"),
		HTTPTimeout: durationDefault(getenv("HTTP_TIMEOUT", "45s"), 45*time.Second),

		DatabaseURL:   os.Getenv("DATABASE_URL"),
		RunMigrations: boolDefault(getenv("RUN_MIGRATIONS", "false"), false),

		S3BucketName:      os.Getenv("S3_BUCKET"),
		S3Region:          getenv("S3_REGION", "us-east-1"),
		S3AccessKeyID:     os.Getenv("AWS_ACCESS_KEY_ID"),
		S3SecretAccessKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),

		GCSBucketName:      os.Getenv("GCS_BUCKET"),
		GCPProjectID:       os.Getenv("GCP_PROJECT_ID"),
		GCSCredentialsJSON: os.Getenv("GCS_CREDENTIALS_JSON"),

		StorageProvider: getenv("STORAGE_PROVIDER", "s3"),

		MaxConcurrency:         atoiDefault(getenv("MAX_CONCURRENCY", "3"), 3),
		RequireAuth:            boolDefault(getenv("AUTH_REQUIRED", "false"), false),
		FirebaseProjectID:      getenv("FIREBASE_PROJECT_ID", getenv("GCP_PROJECT_ID", "")),
		MetaWebhookVerifyToken: os.Getenv("META_WEBHOOK_VERIFY_TOKEN"),
		MetaWebhookAppSecret:   os.Getenv("META_WEBHOOK_APP_SECRET"),

		ContingencyInternalToken:        os.Getenv("CONTINGENCY_INTERNAL_TOKEN"),
		ContingencyMonitorAdAccounts:    csvDefault(os.Getenv("CONTINGENCY_MONITOR_AD_ACCOUNTS")),
		ContingencyDefaultMaxCandidates: atoiDefault(getenv("CONTINGENCY_MAX_CANDIDATES", "50"), 50),
		ContingencyDefaultMaxAttempts:   atoiDefault(getenv("CONTINGENCY_MAX_ATTEMPTS", "3"), 3),
		ContingencyDefaultRefreshStatus: boolDefault(getenv("CONTINGENCY_REFRESH_STATUS", "true"), true),
		ContingencyDispatchViaTasks:     boolDefault(getenv("CONTINGENCY_DISPATCH_VIA_TASKS", "true"), true),
		ContingencyTasksProjectID:       getenv("CONTINGENCY_TASKS_PROJECT_ID", getenv("GCP_PROJECT_ID", "")),
		ContingencyTasksLocation:        getenv("CONTINGENCY_TASKS_LOCATION", "us-central1"),
		ContingencyTasksQueue:           getenv("CONTINGENCY_TASKS_QUEUE", "contingency-executor"),
		ContingencyTasksExecuteURL:      os.Getenv("CONTINGENCY_TASKS_EXECUTE_URL"),
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
