package service

import (
	"context"
	"fmt"
	"time"

	"creative-service/internal/meta"
	"creative-service/internal/secrets"
	"creative-service/internal/storage"
)

type CampaignService struct {
	Store  *storage.Store
	Tokens secrets.Resolver

	BaseURL     string
	APIVersion  string
	HTTPTimeout time.Duration

	Sem *Semaphore
}

type CreateCampaignInput struct {
	ClientID             string // Deprecated: manter por compatibilidade
	AdAccountID          string 
	Name                 string
	Objective            string
	Status               string
	SpecialAdCategories  []string
	BuyingType           string
	IsAdSetBudgetSharingEnabled bool
}

type CreateCampaignOutput struct {
	CampaignID string `json:"campaign_id"`
}

func (s *CampaignService) CreateCampaign(ctx context.Context, in CreateCampaignInput) (CreateCampaignOutput, error) {
	if err := s.Sem.Acquire(ctx); err != nil {
		return CreateCampaignOutput{}, err
	}
	defer s.Sem.Release()

	// Buscar ad account pelo ID (act_123456789)
	adAccount, err := s.Store.GetAdAccount(ctx, in.AdAccountID)
	if err != nil {
		return CreateCampaignOutput{}, fmt.Errorf("get ad account: %w", err)
	}

	token, err := s.Tokens.Resolve(adAccount.TokenRef)
	if err != nil {
		return CreateCampaignOutput{}, fmt.Errorf("resolve token: %w", err)
	}

	mc := meta.New(s.BaseURL, s.APIVersion, token, s.HTTPTimeout)

	payload := map[string]any{
		"name":                              in.Name,
		"objective":                         in.Objective,
		"status":                            in.Status,
		"special_ad_categories":             in.SpecialAdCategories,
		"buying_type":                       in.BuyingType,
		"is_adset_budget_sharing_enabled":   in.IsAdSetBudgetSharingEnabled,
	}

	fmt.Printf("=== PAYLOAD PARA META API ===\n%+v\n", payload)

	campaignID, err := mc.CreateCampaign(ctx, adAccount.AdAccountID, payload)
	if err != nil {
		return CreateCampaignOutput{}, err
	}

	return CreateCampaignOutput{CampaignID: campaignID}, nil
}

type ListCampaignsInput struct {
	AdAccountID string
}

type CampaignItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Objective   string `json:"objective,omitempty"`
	Status      string `json:"status,omitempty"`
	CreatedTime string `json:"created_time,omitempty"`
}

type ListCampaignsOutput struct {
	Campaigns []CampaignItem `json:"campaigns"`
}

func (s *CampaignService) ListCampaigns(ctx context.Context, in ListCampaignsInput) (ListCampaignsOutput, error) {
	if err := s.Sem.Acquire(ctx); err != nil {
		return ListCampaignsOutput{}, err
	}
	defer s.Sem.Release()

	adAccount, err := s.Store.GetAdAccount(ctx, in.AdAccountID)
	if err != nil {
		return ListCampaignsOutput{}, fmt.Errorf("get ad account: %w", err)
	}

	token, err := s.Tokens.Resolve(adAccount.TokenRef)
	if err != nil {
		return ListCampaignsOutput{}, fmt.Errorf("resolve token: %w", err)
	}

	mc := meta.New(s.BaseURL, s.APIVersion, token, s.HTTPTimeout)

	fields := []string{"id", "name", "objective", "status", "created_time"}
	data, err := mc.ListCampaigns(ctx, adAccount.AdAccountID, fields)
	if err != nil {
		return ListCampaignsOutput{}, err
	}

	campaigns := make([]CampaignItem, 0, len(data))
	for _, item := range data {
		c := CampaignItem{}
		if id, ok := item["id"].(string); ok {
			c.ID = id
		}
		if name, ok := item["name"].(string); ok {
			c.Name = name
		}
		if obj, ok := item["objective"].(string); ok {
			c.Objective = obj
		}
		if status, ok := item["status"].(string); ok {
			c.Status = status
		}
		if ct, ok := item["created_time"].(string); ok {
			c.CreatedTime = ct
		}
		campaigns = append(campaigns, c)
	}

	return ListCampaignsOutput{Campaigns: campaigns}, nil
}
