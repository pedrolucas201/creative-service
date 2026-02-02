package service

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"creative-service/internal/blob"
	"creative-service/internal/queue"
	"creative-service/internal/storage"

	"github.com/google/uuid"
)

type VideoJobService struct {
	Store *storage.Store
	Blob  blob.Store
	Queue *queue.Queue
}

type CreateVideoJobInput struct {
	ClientID string

	Name        string
	Link        string
	Message     string
	Headline    string
	Description string
	CTAType     string

	VideoName  string
	VideoBytes []byte
	ThumbName  string
	ThumbBytes []byte
}

type CreateJobOutput struct {
	JobID string `json:"job_id"`
}

func (s *VideoJobService) EnqueueVideoCreativeJob(ctx context.Context, in CreateVideoJobInput) (CreateJobOutput, error) {
	jobID := uuid.NewString()

	videoKey := filepath.Join("jobs", jobID, in.VideoName)
	thumbKey := filepath.Join("jobs", jobID, in.ThumbName)

	videoPath, err := s.Blob.Save(ctx, videoKey, in.VideoBytes)
	if err != nil { return CreateJobOutput{}, fmt.Errorf("save video: %w", err) }
	thumbPath, err := s.Blob.Save(ctx, thumbKey, in.ThumbBytes)
	if err != nil { return CreateJobOutput{}, fmt.Errorf("save thumb: %w", err) }

	input := map[string]any{
		"client_id": in.ClientID,
		"name": in.Name,
		"link": in.Link,
		"message": in.Message,
		"headline": in.Headline,
		"description": in.Description,
		"cta_type": in.CTAType,
		"created_at": time.Now().UTC().Format(time.RFC3339),
	}
	b, _ := json.Marshal(input)

	vp := videoPath
	tp := thumbPath
	job := storage.Job{
		JobID: jobID,
		ClientID: in.ClientID,
		JobType: "creative_video",
		Status: "queued",
		InputJSON: b,
		BlobVideoPath: &vp,
		BlobThumbPath: &tp,
	}

	if err := s.Store.CreateJob(ctx, job); err != nil {
		return CreateJobOutput{}, fmt.Errorf("create job: %w", err)
	}
	if err := s.Queue.Enqueue(ctx, jobID); err != nil {
		return CreateJobOutput{}, fmt.Errorf("enqueue: %w", err)
	}
	return CreateJobOutput{JobID: jobID}, nil
}
