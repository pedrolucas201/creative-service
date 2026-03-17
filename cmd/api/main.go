package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"creative-service/internal/auth"
	"creative-service/internal/automation"
	"creative-service/internal/bm"
	"creative-service/internal/config"
	"creative-service/internal/httpapi"
	"creative-service/internal/s3"
	"creative-service/internal/secrets"
	"creative-service/internal/service"
	"creative-service/internal/storage"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func runMigrations(dbURL string) error {
	m, err := migrate.New(
		"file://internal/storage/migrations",
		dbURL,
	)
	if err != nil {
		return err
	}
	return m.Up()
}

func main() {
	_ = godotenv.Load()

	cfg := config.Load()
	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	if cfg.GCPProjectID == "" {
		log.Fatal("GCP_PROJECT_ID is required")
	}

	if cfg.RunMigrations {
		if err := runMigrations(cfg.DatabaseURL); err != nil {
			log.Printf("Migration warning: %v", err)
		}
	} else {
		log.Println("Skipping migrations at startup (RUN_MIGRATIONS=false)")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	st := storage.New(pool)

	smResolver, err := secrets.NewSMResolver(ctx, cfg.GCPProjectID, 2*time.Minute)
	if err != nil {
		log.Fatal("failed to create SMResolver: ", err)
	}
	defer smResolver.Close()

	bmService := &bm.Service{
		DB: pool,
		SM: smResolver,
	}

	var storageClient storage.StorageClient

	if cfg.StorageProvider == "gcs" {
		log.Println("Initializing GCS client...")
		gcsClient, err := storage.NewGCSClient(ctx, storage.GCSConfig{
			BucketName:      cfg.GCSBucketName,
			ProjectID:       cfg.GCPProjectID,
			CredentialsJSON: cfg.GCSCredentialsJSON,
		})
		if err != nil {
			log.Fatal("failed to create GCS client: ", err)
		}
		defer gcsClient.Close()
		storageClient = gcsClient
		log.Println("GCS client initialized for bucket:", cfg.GCSBucketName)
	} else {
		log.Println("Initializing S3 client...")
		s3Client, err := s3.New(ctx, s3.Config{
			BucketName:      cfg.S3BucketName,
			Region:          cfg.S3Region,
			AccessKeyID:     cfg.S3AccessKeyID,
			SecretAccessKey: cfg.S3SecretAccessKey,
		})
		if err != nil {
			log.Fatal("failed to create S3 client: ", err)
		}
		storageClient = s3Client
		log.Println("S3 client initialized for bucket:", cfg.S3BucketName)
	}

	sem := service.NewSemaphore(cfg.MaxConcurrency)
	tokens := secrets.MultiResolver{
		Env: secrets.EnvResolver{},
		SM:  smResolver,
	}

	creativeSync := &service.CreativeSyncService{
		Store:       st,
		BM:          bmService,
		Tokens:      tokens,
		Storage:     storageClient,
		BaseURL:     cfg.BaseURL,
		APIVersion:  cfg.APIVersion,
		HTTPTimeout: cfg.HTTPTimeout,
		Sem:         sem,
	}

	campaigns := &service.CampaignService{
		Store:       st,
		BM:          bmService,
		Tokens:      tokens,
		BaseURL:     cfg.BaseURL,
		APIVersion:  cfg.APIVersion,
		HTTPTimeout: cfg.HTTPTimeout,
		Sem:         sem,
	}

	adsets := &service.AdSetService{
		Store:       st,
		BM:          bmService,
		Tokens:      tokens,
		BaseURL:     cfg.BaseURL,
		APIVersion:  cfg.APIVersion,
		HTTPTimeout: cfg.HTTPTimeout,
		Sem:         sem,
	}

	ads := &service.AdService{
		Store:       st,
		BM:          bmService,
		Tokens:      tokens,
		BaseURL:     cfg.BaseURL,
		APIVersion:  cfg.APIVersion,
		HTTPTimeout: cfg.HTTPTimeout,
		Sem:         sem,
	}

	var verifier auth.Verifier
	var userManager auth.UserManager
	if cfg.RequireAuth {
		fv, err := auth.NewFirebaseVerifier(ctx, cfg.FirebaseProjectID)
		if err != nil {
			log.Fatal("failed to create firebase verifier: ", err)
		}
		verifier = fv
		userManager = fv
		log.Println("Firebase auth enabled")
	} else {
		log.Println("Firebase auth disabled (AUTH_REQUIRED=false)")
	}

	var contingencyTasks *automation.ContingencyTaskQueue
	if cfg.ContingencyTasksExecuteURL != "" {
		tasks, err := automation.NewContingencyTaskQueue(ctx, automation.ContingencyTaskQueueConfig{
			ProjectID:     cfg.ContingencyTasksProjectID,
			Location:      cfg.ContingencyTasksLocation,
			QueueID:       cfg.ContingencyTasksQueue,
			ExecuteURL:    cfg.ContingencyTasksExecuteURL,
			InternalToken: cfg.ContingencyInternalToken,
		})
		if err != nil {
			log.Fatal("failed to create contingency cloud tasks dispatcher: ", err)
		}
		contingencyTasks = tasks
		defer contingencyTasks.Close()
		log.Println("Contingency Cloud Tasks dispatcher enabled:", cfg.ContingencyTasksQueue)
	} else {
		log.Println("Contingency Cloud Tasks dispatcher disabled (CONTINGENCY_TASKS_EXECUTE_URL is empty)")
	}

	h := &httpapi.Handler{
		CreativeSync:                    creativeSync,
		Store:                           st,
		Campaigns:                       campaigns,
		AdSets:                          adsets,
		Ads:                             ads,
		BM:                              bmService,
		UserManager:                     userManager,
		MetaWebhookVerifyToken:          cfg.MetaWebhookVerifyToken,
		MetaWebhookAppSecret:            cfg.MetaWebhookAppSecret,
		ContingencyTasks:                contingencyTasks,
		ContingencyInternalToken:        cfg.ContingencyInternalToken,
		ContingencyMonitorAdAccounts:    cfg.ContingencyMonitorAdAccounts,
		ContingencyDefaultMaxCandidates: cfg.ContingencyDefaultMaxCandidates,
		ContingencyDefaultMaxAttempts:   cfg.ContingencyDefaultMaxAttempts,
		ContingencyDefaultRefreshStatus: cfg.ContingencyDefaultRefreshStatus,
		ContingencyDispatchViaTasks:     cfg.ContingencyDispatchViaTasks,
	}

	router := httpapi.NewRouter(h, httpapi.RouterOptions{
		RequireAuth:  cfg.RequireAuth,
		AuthVerifier: verifier,
		AppUserStore: st,
	})

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Println("api listening on", cfg.Addr, "pid", os.Getpid())
	log.Fatal(server.ListenAndServe())
}
