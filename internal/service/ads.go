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
	ClientID    string // Deprecated: manter por compatibilidade
	AdAccountID string // Meta ID da ad account (act_123456789)
	AdSetID     string
	CreativeID  string
	Name        string
	Status      string
}

type CreateAdOutput struct {
	AdID string `json:"ad_id"`
}

func (s *AdService) CreateAd(ctx context.Context, in CreateAdInput) (CreateAdOutput, error) {
	if err := s.Sem.Acquire(ctx); err != nil {
		return CreateAdOutput{}, err
	}
	defer s.Sem.Release()

	// Buscar ad account pelo ID (act_123456789)
	adAccount, err := s.Store.GetAdAccount(ctx, in.AdAccountID)
	if err != nil {
		return CreateAdOutput{}, fmt.Errorf("get ad account: %w", err)
	}

	token, err := s.Tokens.Resolve(adAccount.TokenRef)
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

	adID, err := mc.CreateAd(ctx, adAccount.AdAccountID, payload)
	if err != nil {
		return CreateAdOutput{}, err
	}

	return CreateAdOutput{AdID: adID}, nil
}

type ListAdsInput struct {
	AdAccountID string
}

type AdItem struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	AdSetID     string         `json:"adset_id,omitempty"`
	Status      string         `json:"status,omitempty"`
	Creative    map[string]any `json:"creative,omitempty"`
	CreatedTime string         `json:"created_time,omitempty"`
}

type ListAdsOutput struct {
	Ads []AdItem `json:"ads"`
}

func (s *AdService) ListAds(ctx context.Context, in ListAdsInput) (ListAdsOutput, error) {
	if err := s.Sem.Acquire(ctx); err != nil {
		return ListAdsOutput{}, err
	}
	defer s.Sem.Release()

	adAccount, err := s.Store.GetAdAccount(ctx, in.AdAccountID)
	if err != nil {
		return ListAdsOutput{}, fmt.Errorf("get ad account: %w", err)
	}

	token, err := s.Tokens.Resolve(adAccount.TokenRef)
	if err != nil {
		return ListAdsOutput{}, fmt.Errorf("resolve token: %w", err)
	}

	mc := meta.New(s.BaseURL, s.APIVersion, token, s.HTTPTimeout)

	fields := []string{"id", "name", "adset_id", "status", "creative{id,name}", "created_time"}
	data, err := mc.ListAds(ctx, adAccount.AdAccountID, fields)
	if err != nil {
		return ListAdsOutput{}, err
	}

	ads := make([]AdItem, 0, len(data))
	for _, item := range data {
		a := AdItem{}
		if id, ok := item["id"].(string); ok {
			a.ID = id
		}
		if name, ok := item["name"].(string); ok {
			a.Name = name
		}
		if asid, ok := item["adset_id"].(string); ok {
			a.AdSetID = asid
		}
		if status, ok := item["status"].(string); ok {
			a.Status = status
		}
		if creative, ok := item["creative"].(map[string]any); ok {
			a.Creative = creative
		}
		if ct, ok := item["created_time"].(string); ok {
			a.CreatedTime = ct
		}
		ads = append(ads, a)
	}

	return ListAdsOutput{Ads: ads}, nil
}
