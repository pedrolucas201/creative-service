package service

import (
	"context"
	"fmt"
	"time"

	"creative-service/internal/meta"
	"creative-service/internal/secrets"
	"creative-service/internal/storage"
)

type AdService struct {
	Store  *storage.Store
	Tokens secrets.Resolver

	BaseURL     string
	APIVersion  string
	HTTPTimeout time.Duration

	Sem *Semaphore
}

type CreateAdInput struct {
	ClientID   string
	AdSetID    string
	CreativeID string
	Name       string
	Status     string
}

type CreateAdOutput struct {
	AdID string `json:"ad_id"`
}

func (s *AdService) CreateAd(ctx context.Context, in CreateAdInput) (CreateAdOutput, error) {
	if err := s.Sem.Acquire(ctx); err != nil {
		return CreateAdOutput{}, err
	}
	defer s.Sem.Release()

	client, err := s.Store.GetClient(ctx, in.ClientID)
	if err != nil {
		return CreateAdOutput{}, fmt.Errorf("get client: %w", err)
	}

	token, err := s.Tokens.Resolve(client.TokenRef)
	if err != nil {
		return CreateAdOutput{}, fmt.Errorf("resolve token: %w", err)
	}

	mc := meta.New(s.BaseURL, s.APIVersion, token, s.HTTPTimeout)

	payload := map[string]any{
		"name":       in.Name,
		"adset_id":   in.AdSetID,
		"creative":   map[string]any{"creative_id": in.CreativeID},
		"status":     in.Status,
	}

	adID, err := mc.CreateAd(ctx, client.AdAccountID, payload)
	if err != nil {
		return CreateAdOutput{}, err
	}

	return CreateAdOutput{AdID: adID}, nil
}
