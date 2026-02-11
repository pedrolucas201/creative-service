package service

import (
	"context"
	"fmt"
	"time"

	"creative-service/internal/meta"
	"creative-service/internal/secrets"
	"creative-service/internal/storage"
)

type AdSetService struct {
	Store  *storage.Store
	Tokens secrets.Resolver

	BaseURL     string
	APIVersion  string
	HTTPTimeout time.Duration

	Sem *Semaphore
}

type CreateAdSetInput struct {
	ClientID         string // Deprecated: manter por compatibilidade
	AdAccountID      string // Meta ID da ad account (act_123456789)
	CampaignID       string
	Name             string
	BillingEvent     string
	OptimizationGoal string
	BidAmount        int
	DailyBudget      int
	Targeting        map[string]any
	Status           string
}

type CreateAdSetOutput struct {
	AdSetID string `json:"adset_id"`
}

func (s *AdSetService) CreateAdSet(ctx context.Context, in CreateAdSetInput) (CreateAdSetOutput, error) {
	if err := s.Sem.Acquire(ctx); err != nil {
		return CreateAdSetOutput{}, err
	}
	defer s.Sem.Release()

	// Buscar ad account pelo ID (act_123456789)
	adAccount, err := s.Store.GetAdAccount(ctx, in.AdAccountID)
	if err != nil {
		return CreateAdSetOutput{}, fmt.Errorf("get ad account: %w", err)
	}

	token, err := s.Tokens.Resolve(adAccount.TokenRef)
	if err != nil {
		return CreateAdSetOutput{}, fmt.Errorf("resolve token: %w", err)
	}

	mc := meta.New(s.BaseURL, s.APIVersion, token, s.HTTPTimeout)

	payload := map[string]any{
		"campaign_id":       in.CampaignID,
		"name":              in.Name,
		"billing_event":     in.BillingEvent,
		"optimization_goal": in.OptimizationGoal,
		"bid_strategy":      "LOWEST_COST_WITHOUT_CAP",
		"daily_budget":      in.DailyBudget,
		"targeting":         in.Targeting,
		"status":            in.Status,
	}

	adsetID, err := mc.CreateAdSet(ctx, adAccount.AdAccountID, payload)
	if err != nil {
		return CreateAdSetOutput{}, err
	}

	return CreateAdSetOutput{AdSetID: adsetID}, nil
}

type ListAdSetsInput struct {
	AdAccountID string
}

type AdSetItem struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	CampaignID      string `json:"campaign_id,omitempty"`
	Status          string `json:"status,omitempty"`
	DailyBudget     string `json:"daily_budget,omitempty"`
	BillingEvent    string `json:"billing_event,omitempty"`
	CreatedTime     string `json:"created_time,omitempty"`
}

type ListAdSetsOutput struct {
	AdSets []AdSetItem `json:"adsets"`
}

func (s *AdSetService) ListAdSets(ctx context.Context, in ListAdSetsInput) (ListAdSetsOutput, error) {
	if err := s.Sem.Acquire(ctx); err != nil {
		return ListAdSetsOutput{}, err
	}
	defer s.Sem.Release()

	adAccount, err := s.Store.GetAdAccount(ctx, in.AdAccountID)
	if err != nil {
		return ListAdSetsOutput{}, fmt.Errorf("get ad account: %w", err)
	}

	token, err := s.Tokens.Resolve(adAccount.TokenRef)
	if err != nil {
		return ListAdSetsOutput{}, fmt.Errorf("resolve token: %w", err)
	}

	mc := meta.New(s.BaseURL, s.APIVersion, token, s.HTTPTimeout)

	fields := []string{"id", "name", "campaign_id", "status", "daily_budget", "billing_event", "created_time"}
	data, err := mc.ListAdSets(ctx, adAccount.AdAccountID, fields)
	if err != nil {
		return ListAdSetsOutput{}, err
	}

	adsets := make([]AdSetItem, 0, len(data))
	for _, item := range data {
		a := AdSetItem{}
		if id, ok := item["id"].(string); ok {
			a.ID = id
		}
		if name, ok := item["name"].(string); ok {
			a.Name = name
		}
		if cid, ok := item["campaign_id"].(string); ok {
			a.CampaignID = cid
		}
		if status, ok := item["status"].(string); ok {
			a.Status = status
		}
		if db, ok := item["daily_budget"].(string); ok {
			a.DailyBudget = db
		}
		if be, ok := item["billing_event"].(string); ok {
			a.BillingEvent = be
		}
		if ct, ok := item["created_time"].(string); ok {
			a.CreatedTime = ct
		}
		adsets = append(adsets, a)
	}

	return ListAdSetsOutput{AdSets: adsets}, nil
}
