package service

import (
	"context"
	"encoding/json"
	"time"

	"creative-service/internal/blob"
	"creative-service/internal/meta"
	"creative-service/internal/secrets"
	"creative-service/internal/storage"
)

type VideoJobProcessor struct {
	Store  *storage.Store
	Blob   blob.Store
	Tokens secrets.Resolver

	BaseURL     string
	APIVersion  string
	HTTPTimeout time.Duration

	Sem *Semaphore
}

type videoJobInput struct {
	ClientID     string `json:"client_id"`
	Name         string `json:"name"`
	Link         string `json:"link"`
	Message      string `json:"message"`
	Headline     string `json:"headline"`
	Description  string `json:"description"`
	CTAType      string `json:"cta_type"`
}

type VideoJobResult struct {
	VideoID        string `json:"video_id"`
	ThumbnailHash  string `json:"thumbnail_hash"`
	CreativeID     string `json:"creative_id"`
	Validated      bool   `json:"validated"`
}

func (p *VideoJobProcessor) Process(ctx context.Context, jobID string) {
	if err := p.Sem.Acquire(ctx); err != nil { return }
	defer p.Sem.Release()

	_ = p.Store.UpdateJobStatus(ctx, jobID, "running")

	job, err := p.Store.GetJob(ctx, jobID)
	if err != nil {
		_ = p.Store.FailJob(ctx, jobID, "get job: "+err.Error()); return
	}

	var in videoJobInput
	if err := json.Unmarshal(job.InputJSON, &in); err != nil {
		_ = p.Store.FailJob(ctx, jobID, "bad input_json: "+err.Error()); return
	}

	client, err := p.Store.GetClient(ctx, job.ClientID)
	if err != nil { _ = p.Store.FailJob(ctx, jobID, "get client: "+err.Error()); return }

	token, err := p.Tokens.Resolve(client.TokenRef)
	if err != nil { _ = p.Store.FailJob(ctx, jobID, "resolve token: "+err.Error()); return }

	if job.BlobVideoPath == nil || job.BlobThumbPath == nil {
		_ = p.Store.FailJob(ctx, jobID, "missing blob paths"); return
	}

	videoBytes, err := p.Blob.Load(ctx, *job.BlobVideoPath)
	if err != nil { _ = p.Store.FailJob(ctx, jobID, "load video: "+err.Error()); return }
	thumbBytes, err := p.Blob.Load(ctx, *job.BlobThumbPath)
	if err != nil { _ = p.Store.FailJob(ctx, jobID, "load thumb: "+err.Error()); return }

	mc := meta.New(p.BaseURL, p.APIVersion, token, p.HTTPTimeout)

	videoID, err := mc.UploadVideo(ctx, client.AdAccountID, in.Name, "video.mp4", videoBytes)
	if err != nil { _ = p.Store.FailJob(ctx, jobID, "upload video: "+err.Error()); return }

	thumbHash, err := mc.UploadImage(ctx, client.AdAccountID, "thumb.png", thumbBytes)
	if err != nil { _ = p.Store.FailJob(ctx, jobID, "upload thumb: "+err.Error()); return }

	cta := in.CTAType
	if cta == "" { cta = "LEARN_MORE" }

	payload := map[string]any{
		"name": in.Name,
		"object_story_spec": map[string]any{
			"page_id": client.PageID,
			"video_data": map[string]any{
				"video_id": videoID,
				"image_hash": thumbHash,
				"message": in.Message,
				"title": in.Headline,
				"link_description": in.Description,
				"call_to_action": map[string]any{
					"type": cta,
					"value": map[string]any{"link": in.Link},
				},
			},
		},
	}

	creativeID, err := mc.CreateCreative(ctx, client.AdAccountID, payload)
	if err != nil { _ = p.Store.FailJob(ctx, jobID, "create creative: "+err.Error()); return }

	_, err = mc.GetCreative(ctx, creativeID, []string{"id", "object_story_spec"})
	if err != nil { _ = p.Store.FailJob(ctx, jobID, "validate creative: "+err.Error()); return }

	res := VideoJobResult{VideoID: videoID, ThumbnailHash: thumbHash, CreativeID: creativeID, Validated: true}
	if err := p.Store.CompleteJob(ctx, jobID, res); err != nil {
		_ = p.Store.FailJob(ctx, jobID, "complete job: "+err.Error()); return
	}
}
