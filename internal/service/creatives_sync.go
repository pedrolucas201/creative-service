package service

import (
	"context"
	"fmt"
	"time"

	"creative-service/internal/meta"
	"creative-service/internal/secrets"
	"creative-service/internal/storage"
)

type CreativeSyncService struct {
	Store  *storage.Store
	Tokens secrets.Resolver

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
	Validated  bool   `json:"validated"`
}

func (s *CreativeSyncService) CreateImageCreative(ctx context.Context, in ImageCreativeInput) (ImageCreativeOutput, error) {
	if err := s.Sem.Acquire(ctx); err != nil { return ImageCreativeOutput{}, err }
	defer s.Sem.Release()

	client, err := s.Store.GetClient(ctx, in.ClientID)
	if err != nil { return ImageCreativeOutput{}, fmt.Errorf("get client: %w", err) }

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

	return ImageCreativeOutput{ImageHash: imageHash, CreativeID: creativeID, Validated: true}, nil
}
