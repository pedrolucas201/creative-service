package service

import (
	"context"
	"fmt"
	"time"
	"bytes"
	"encoding/json"

	"creative-service/internal/bm"
	"creative-service/internal/meta"
	"creative-service/internal/secrets"
	"creative-service/internal/storage"

	"github.com/google/uuid"
)

type CreativeSyncService struct {
	Store  *storage.Store
	BM     *bm.Service
	Tokens secrets.Resolver
	Storage storage.StorageClient // Interface genérica (S3 ou GCS)

	BaseURL     string
	APIVersion  string
	HTTPTimeout time.Duration

	Sem *Semaphore
}

type ImageCreativeInput struct {
	AdAccountID string // Meta ID da ad account (act_123456789)

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
	URL        string `json:"url"`
	Validated  bool   `json:"validated"`
}

type VideoCreativeInput struct {
	AdAccountID string // Meta ID da ad account (act_123456789)
	
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
	VideoURL   string `json:"video_url"`
	ThumbURL   string `json:"thumb_url"`
	Validated  bool   `json:"validated"`
}

func (s *CreativeSyncService) CreateImageCreative(ctx context.Context, in ImageCreativeInput) (ImageCreativeOutput, error) {
	if err := s.Sem.Acquire(ctx); err != nil { return ImageCreativeOutput{}, err }
	defer s.Sem.Release()

	// Buscar ad account pelo ID (act_123456789)
	adAccount, err := s.Store.GetAdAccount(ctx, in.AdAccountID)
	if err != nil { return ImageCreativeOutput{}, fmt.Errorf("get ad account: %w", err) }

	// Buscar client para pegar nome (usado no path S3)
	client, err := s.Store.GetClientByUUID(ctx, adAccount.ClientUUID)
	if err != nil { return ImageCreativeOutput{}, fmt.Errorf("get client: %w", err) }

	// Gerar UUID único para o creative
	creativeUUID := uuid.New().String()
	
	// Nova estrutura S3: creatives/images/{client_uuid}-{client_name}/{ad_account_id}-{ad_account_name}/{creative_uuid}-{filename}
	clientName := "unknown"
	if client.Name != "" {
		clientName = client.Name
	}
	imageKey := fmt.Sprintf("creatives/images/%s-%s/%s-%s/%s-%s", 
		client.ClientUUID, clientName, adAccount.AdAccountID, adAccount.AdAccountName, creativeUUID, in.ImageName)
	
	imageReader := bytes.NewReader(in.ImageBytes)
	imagePath, err := s.Storage.Upload(ctx, imageKey, imageReader, "image/jpeg")
	if err != nil { return ImageCreativeOutput{}, fmt.Errorf("upload to storage: %w", err) }
	imageURL := s.Storage.GetURL(imagePath)

	bmCfg, err := s.BM.GetBMConfigByAdAccountID(ctx, adAccount.AdAccountID)
	if err != nil { return ImageCreativeOutput{}, fmt.Errorf("get bm config: %w", err) }

	token, err := s.Tokens.Resolve(bmCfg.TokenRef)
	if err != nil { return ImageCreativeOutput{}, fmt.Errorf("resolve token: %w", err) }

	mc := meta.New(s.BaseURL, s.APIVersion, token, s.HTTPTimeout)

	imageHash, err := mc.UploadImage(ctx, adAccount.AdAccountID, in.ImageName, in.ImageBytes)
	if err != nil { return ImageCreativeOutput{}, err }

	payload := map[string]any{
		"name": in.Name,
		"object_story_spec": map[string]any{
				"page_id": bmCfg.PageID,
			"link_data": map[string]any{
				"image_hash":  imageHash,
				"link":        in.Link,
				"message":     in.Message,
				"name":        in.Headline,
				"description": in.Description,
			},
		},
	}

	creativeID, err := mc.CreateCreative(ctx, adAccount.AdAccountID, payload)
	if err != nil { return ImageCreativeOutput{}, err }

	_, err = mc.GetCreative(ctx, creativeID, []string{"id", "object_story_spec"})
	if err != nil { return ImageCreativeOutput{}, fmt.Errorf("creative created but validate failed: %w", err) }

	creativeRecord := storage.Creative{
		CreativeID:  creativeID,
		ClientUUID:  client.ClientUUID,
		AdAccountID: adAccount.AdAccountID,
		Name:        in.Name,
		Type:        "image",
		URL:         imageURL,
		ThumbURL:    nil,
		Link:        &in.Link,
		Message:     &in.Message,
		MetaData:    json.RawMessage(fmt.Sprintf(`{"image_hash":"%s"}`, imageHash)),
	}

	if err := s.Store.CreateCreative(ctx, creativeRecord); err != nil {
		return ImageCreativeOutput{}, fmt.Errorf("save creative to DB: %w", err)
	}

	return ImageCreativeOutput{ImageHash: imageHash, CreativeID: creativeID, URL: imageURL, Validated: true}, nil
}

func (s *CreativeSyncService) CreateVideoCreative(ctx context.Context, in VideoCreativeInput) (VideoCreativeOutput, error) {
	if err := s.Sem.Acquire(ctx); err != nil { return VideoCreativeOutput{}, err }
	defer s.Sem.Release()

	// Buscar ad account pelo ID (act_123456789)
	adAccount, err := s.Store.GetAdAccount(ctx, in.AdAccountID)
	if err != nil { return VideoCreativeOutput{}, fmt.Errorf("get ad account: %w", err) }

	// Buscar client para pegar nome (usado no path S3)
	client, err := s.Store.GetClientByUUID(ctx, adAccount.ClientUUID)
	if err != nil { return VideoCreativeOutput{}, fmt.Errorf("get client: %w", err) }

	bmCfg, err := s.BM.GetBMConfigByAdAccountID(ctx, adAccount.AdAccountID)
	if err != nil { return VideoCreativeOutput{}, fmt.Errorf("get bm config: %w", err) }

	token, err := s.Tokens.Resolve(bmCfg.TokenRef)
	if err != nil { return VideoCreativeOutput{}, fmt.Errorf("resolve token: %w", err) }

	// Gerar UUID único para o creative
	creativeUUID := uuid.New().String()
	
	// Nova estrutura S3: creatives/videos/{client_uuid}-{client_name}/{ad_account_id}-{ad_account_name}/{creative_uuid}-{filename}
	clientName := "unknown"
	if client.Name != "" {
		clientName = client.Name
	}
	videoKey := fmt.Sprintf("creatives/videos/%s-%s/%s-%s/%s-%s", 
		client.ClientUUID, clientName, adAccount.AdAccountID, adAccount.AdAccountName, creativeUUID, in.VideoName)
	videoReader := bytes.NewReader(in.VideoBytes)
	videoPath, err := s.Storage.Upload(ctx, videoKey, videoReader, "video/mp4")

	if err != nil {
		return VideoCreativeOutput{}, fmt.Errorf("upload video to storage: %w", err)
	}
	videoURL := s.Storage.GetURL(videoPath)

	thumbKey := fmt.Sprintf("creatives/thumbnails/%s-%s/%s-%s/%s-thumb-%s", 
		client.ClientUUID, clientName, adAccount.AdAccountID, adAccount.AdAccountName, creativeUUID, in.ThumbName)
   	thumbReader := bytes.NewReader(in.ThumbBytes)
	thumbPath, err := s.Storage.Upload(ctx, thumbKey, thumbReader, "image/jpeg")

	if err != nil {
		return VideoCreativeOutput{}, fmt.Errorf("upload thumb to storage: %w", err)
	}
	thumbURL := s.Storage.GetURL(thumbPath)

	mc := meta.New(s.BaseURL, s.APIVersion, token, s.HTTPTimeout)

   	videoID, err := mc.UploadVideo(ctx, adAccount.AdAccountID, in.Name, in.VideoName, in.VideoBytes)
   	if err != nil { return VideoCreativeOutput{}, fmt.Errorf("upload video to Meta: %w", err) }

	imageHash, err := mc.UploadImage(ctx, adAccount.AdAccountID, in.ThumbName, in.ThumbBytes)
	if err != nil { return VideoCreativeOutput{}, fmt.Errorf("upload thumb to Meta: %w", err) }

	payload := map[string]any{
		"name": in.Name,
		"object_story_spec": map[string]any{
			"page_id": bmCfg.PageID,
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

	creativeID, err := mc.CreateCreative(ctx, adAccount.AdAccountID, payload)
	if err != nil { return VideoCreativeOutput{}, err }

	_, err = mc.GetCreative(ctx, creativeID, []string{"id", "object_story_spec"})
	if err != nil { return VideoCreativeOutput{}, fmt.Errorf("creative created but validate failed: %w", err) }

	creativeRecord := storage.Creative{
		CreativeID:  creativeID,
		ClientUUID:  client.ClientUUID,
		AdAccountID: adAccount.AdAccountID,
		Name:        in.Name,
		Type:        "video",
		URL:         videoURL,
		ThumbURL:    &thumbURL,
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
		VideoURL:   videoURL,
		ThumbURL:   thumbURL,
		Validated:  true,
	}, nil

}
