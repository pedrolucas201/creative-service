package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"creative-service/internal/config"
	"creative-service/internal/httpapi"
	"creative-service/internal/s3"
	"creative-service/internal/secrets"
	"creative-service/internal/service"
	"creative-service/internal/storage"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg := config.Load()
	if cfg.DatabaseURL == "" { log.Fatal("DATABASE_URL is required") }

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil { log.Fatal(err) }
	defer pool.Close()

	st := storage.New(pool)
	s3Client, err := s3.New(ctx, s3.Config{
		BucketName:      cfg.S3BucketName,
		Region:          cfg.S3Region,
		AccessKeyID:     cfg.S3AccessKeyID,
		SecretAccessKey: cfg.S3SecretAccessKey,
	})
	if err != nil {
		log.Fatal("failed to create S3 client: ", err)
	}
	log.Println("S3 client initialized for bucket:", cfg.S3BucketName)

	sem := service.NewSemaphore(cfg.MaxConcurrency)
	tokens := secrets.EnvResolver{}

	creativeSync := &service.CreativeSyncService{
		Store: st,
		Tokens: tokens,
		S3: s3Client,
		BaseURL: cfg.BaseURL,
		APIVersion: cfg.APIVersion,
		HTTPTimeout: cfg.HTTPTimeout,
		Sem: sem,
	}

	campaigns := &service.CampaignService{
		Store: st,
		Tokens: tokens,
		BaseURL: cfg.BaseURL,
		APIVersion: cfg.APIVersion,
		HTTPTimeout: cfg.HTTPTimeout,
		Sem: sem,
	}

	adsets := &service.AdSetService{
		Store: st,
		Tokens: tokens,
		BaseURL: cfg.BaseURL,
		APIVersion: cfg.APIVersion,
		HTTPTimeout: cfg.HTTPTimeout,
		Sem: sem,
	}

	ads := &service.AdService{
		Store: st,
		Tokens: tokens,
		BaseURL: cfg.BaseURL,
		APIVersion: cfg.APIVersion,
		HTTPTimeout: cfg.HTTPTimeout,
		Sem: sem,
	}

	h := &httpapi.Handler{
		CreativeSync: creativeSync,
		Store: st,
		Campaigns: campaigns,
		AdSets: adsets,
		Ads: ads,
	}
	router := httpapi.NewRouter(h)

	server := &http.Server{
		Addr: cfg.Addr,
		Handler: router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Println("api listening on", cfg.Addr, "pid", os.Getpid())
	log.Fatal(server.ListenAndServe())
}
