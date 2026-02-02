package main

import (
	"context"
	"log"
	"time"

	"creative-service/internal/blob"
	"creative-service/internal/config"
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
	processor := &service.VideoJobProcessor{
		Store: st,
		Blob: blobStore,
		Tokens: secrets.EnvResolver{},
		BaseURL: cfg.BaseURL,
		APIVersion: cfg.APIVersion,
		HTTPTimeout: cfg.HTTPTimeout,
		Sem: sem,
	}

	log.Println("worker started, queue =", cfg.RedisQueue)
	for {
		jobID, err := q.Dequeue(ctx, 30*time.Second)
		if err != nil { continue }
		if jobID == "" { continue }
		go processor.Process(ctx, jobID)
	}
}
