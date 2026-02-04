package service

import (
	"context"
	"fmt"
	"time"
	"bytes"
	"encoding/json"

	"creative-service/internal/meta"
	"creative-service/internal/s3"
	"creative-service/internal/secrets"
	"creative-service/internal/storage"

	"github.com/google/uuid"
)

type CreativeSyncService struct {
	Store  *storage.Store
	Tokens secrets.Resolver
	S3 *s3.Client

	BaseURL     string
	APIVersion  string
	HTTPTimeout time.Duration

	Sem *Semaphore
}

type ImageCreativeInput struct {
	ClientID string

	Name        string
	Link        string
	Message     string
	Headline    string
	Description string

	ImageName  string
	ImageBytes []byte
}

type ImageCreativeOutput struct {
	ImageHash  string `json:"image_hash"`
	CreativeID string `json:"creative_id"`
	S3URL	   string `json:"s3_url"`
	Validated  bool   `json:"validated"`
}

type VideoCreativeInput struct {
	ClientID    string
	
	Name        string
	Link        string
	Message     string
	Headline    string
	Description string

	VideoName  string
	VideoBytes []byte

	ThumbName  string
	ThumbBytes []byte
}

type VideoCreativeOutput struct {
	VideoID    string `json:"video_id"`
	CreativeID string `json:"creative_id"`
	S3VideoURL	   string `json:"s3_video_url"`
	S3ThumbURL string `json:"s3_thumb_url"`
	Validated  bool   `json:"validated"`
}

func (s *CreativeSyncService) CreateImageCreative(ctx context.Context, in ImageCreativeInput) (ImageCreativeOutput, error) {
	if err := s.Sem.Acquire(ctx); err != nil { return ImageCreativeOutput{}, err }
	defer s.Sem.Release()

	client, err := s.Store.GetClient(ctx, in.ClientID)
	if err != nil { return ImageCreativeOutput{}, fmt.Errorf("get client: %w", err) }

	uid := uuid.New().String()
	imageKey := fmt.Sprintf("creatives/images/%s/%s-%s", in.ClientID, uid, in.ImageName)
	imageReader := bytes.NewReader(in.ImageBytes)
	s3URL, err := s.S3.Upload(ctx, imageKey, imageReader, "image/jpeg")
	if err != nil { return ImageCreativeOutput{}, fmt.Errorf("upload to S3: %w", err) }

	token, err := s.Tokens.Resolve(client.TokenRef)
	if err != nil { return ImageCreativeOutput{}, fmt.Errorf("resolve token: %w", err) }

	mc := meta.New(s.BaseURL, s.APIVersion, token, s.HTTPTimeout)

	imageHash, err := mc.UploadImage(ctx, client.AdAccountID, in.ImageName, in.ImageBytes)
	if err != nil { return ImageCreativeOutput{}, err }

	payload := map[string]any{
		"name": in.Name,
		"object_story_spec": map[string]any{
			"page_id": client.PageID,
			"link_data": map[string]any{
				"image_hash":  imageHash,
				"link":        in.Link,
				"message":     in.Message,
				"name":        in.Headline,
				"description": in.Description,
			},
		},
	}

	creativeID, err := mc.CreateCreative(ctx, client.AdAccountID, payload)
	if err != nil { return ImageCreativeOutput{}, err }

	_, err = mc.GetCreative(ctx, creativeID, []string{"id", "object_story_spec"})
	if err != nil { return ImageCreativeOutput{}, fmt.Errorf("creative created but validate failed: %w", err) }

	creativeRecord := storage.Creative{
		CreativeID:  creativeID,
		ClientID:    in.ClientID,
		Name:        in.Name,
		Type:        "image",
		S3URL:       s3URL,
		S3ThumbURL:  nil,
		Link:        &in.Link,
		Message:     &in.Message,
		MetaData:    json.RawMessage(fmt.Sprintf(`{"image_hash":"%s"}`, imageHash)),
	}

	if err := s.Store.CreateCreative(ctx, creativeRecord); err != nil {
		return ImageCreativeOutput{}, fmt.Errorf("save creative to DB: %w", err)
	}

	return ImageCreativeOutput{ImageHash: imageHash, CreativeID: creativeID, S3URL: s3URL, Validated: true}, nil
}

func (s *CreativeSyncService) CreateVideoCreative(ctx context.Context, in VideoCreativeInput) (VideoCreativeOutput, error) {
	if err := s.Sem.Acquire(ctx); err != nil { return VideoCreativeOutput{}, err }
	defer s.Sem.Release()

	client, err := s.Store.GetClient(ctx, in.ClientID)
	if err != nil { return VideoCreativeOutput{}, fmt.Errorf("get client: %w", err) }

	token, err := s.Tokens.Resolve(client.TokenRef)
	if err != nil { return VideoCreativeOutput{}, fmt.Errorf("resolve token: %w", err) }

	videoUID := uuid.New().String()
	videoKey := fmt.Sprintf("creatives/videos/%s/%s-%s", in.ClientID, videoUID, in.VideoName)
	videoReader := bytes.NewReader(in.VideoBytes)
	s3URL, err := s.S3.Upload(ctx, videoKey, videoReader, "video/mp4")

	if err != nil {
		return VideoCreativeOutput{}, fmt.Errorf("upload video to S3: %w", err)
	}

	thumbUID := uuid.New().String()
   	thumbKey := fmt.Sprintf("creatives/thumbnails/%s/%s-%s", in.ClientID, thumbUID, in.ThumbName)
   	thumbReader := bytes.NewReader(in.ThumbBytes)
	s3ThumbURL, err := s.S3.Upload(ctx, thumbKey, thumbReader, "image/jpeg")

	if err != nil {
		return VideoCreativeOutput{}, fmt.Errorf("upload thumb to S3: %w", err)
	}

	mc := meta.New(s.BaseURL, s.APIVersion, token, s.HTTPTimeout)

   	videoID, err := mc.UploadVideo(ctx, client.AdAccountID, in.Name, in.VideoName, in.VideoBytes)
   	if err != nil { return VideoCreativeOutput{}, fmt.Errorf("upload video to Meta: %w", err) }

	imageHash, err := mc.UploadImage(ctx, client.AdAccountID, in.ThumbName, in.ThumbBytes)
	if err != nil { return VideoCreativeOutput{}, fmt.Errorf("upload thumb to Meta: %w", err) }

	payload := map[string]any{
		"name": in.Name,
		"object_story_spec": map[string]any{
			"page_id": client.PageID,
			"video_data": map[string]any{
				"video_id":    videoID,
				"image_hash":  imageHash,
				"call_to_action": map[string]any{
					"type": "LEARN_MORE",
					"value": map[string]any{
						"link": in.Link,
					},
				},
				"message":     in.Message,
				"title":        in.Headline,
			},
		},
	}

	creativeID, err := mc.CreateCreative(ctx, client.AdAccountID, payload)
	if err != nil { return VideoCreativeOutput{}, err }

	_, err = mc.GetCreative(ctx, creativeID, []string{"id", "object_story_spec"})
	if err != nil { return VideoCreativeOutput{}, fmt.Errorf("creative created but validate failed: %w", err) }

	creativeRecord := storage.Creative{
		CreativeID:  creativeID,
		ClientID:    in.ClientID,
		Name:        in.Name,
		Type:        "video",
		S3URL:       s3URL,
		S3ThumbURL:  &s3ThumbURL,
		Link:        &in.Link,
		Message:     &in.Message,
		MetaData:    json.RawMessage(fmt.Sprintf(`{"video_id":"%s","image_hash":"%s"}`, videoID, imageHash)),
	}

	if err := s.Store.CreateCreative(ctx, creativeRecord); err != nil {
		return VideoCreativeOutput{}, fmt.Errorf("save creative to DB: %w", err)
	}

	return VideoCreativeOutput{
		VideoID:    videoID,
		CreativeID: creativeID,
		S3VideoURL:      s3URL,
		S3ThumbURL: s3ThumbURL,
		Validated:  true,
	}, nil

}