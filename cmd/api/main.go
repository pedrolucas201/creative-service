package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"creative-service/internal/blob"
	"creative-service/internal/config"
	"creative-service/internal/httpapi"
	"creative-service/internal/queue"
	"creative-service/internal/secrets"
	"creative-service/internal/service"
	"creative-service/internal/storage"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func main() {
	cfg := config.Load()
	if cfg.DatabaseURL == "" { log.Fatal("DATABASE_URL is required") }

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil { log.Fatal(err) }
	defer pool.Close()

	st := storage.New(pool)
	blobStore := blob.NewLocalFS(cfg.BlobDir)

	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	if err := rdb.Ping(ctx).Err(); err != nil { log.Fatal("redis ping: ", err) }
	q := queue.New(rdb, cfg.RedisQueue)

	sem := service.NewSemaphore(cfg.MaxConcurrency)
	tokens := secrets.EnvResolver{}

	creativeSync := &service.CreativeSyncService{
		Store: st,
		Tokens: tokens,
		BaseURL: cfg.BaseURL,
		APIVersion: cfg.APIVersion,
		HTTPTimeout: cfg.HTTPTimeout,
		Sem: sem,
	}
	videoJobs := &service.VideoJobService{Store: st, Blob: blobStore, Queue: q}

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
		VideoJobs: videoJobs,
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
