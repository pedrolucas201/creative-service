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
	ClientID         string
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

	client, err := s.Store.GetClient(ctx, in.ClientID)
	if err != nil {
		return CreateAdSetOutput{}, fmt.Errorf("get client: %w", err)
	}

	token, err := s.Tokens.Resolve(client.TokenRef)
	if err != nil {
		return CreateAdSetOutput{}, fmt.Errorf("resolve token: %w", err)
	}

	mc := meta.New(s.BaseURL, s.APIVersion, token, s.HTTPTimeout)

	payload := map[string]any{
		"campaign_id":       in.CampaignID,
		"name":              in.Name,
		"billing_event":     in.BillingEvent,
		"optimization_goal": in.OptimizationGoal,
		"bid_amount":        in.BidAmount,
		"daily_budget":      in.DailyBudget,
		"targeting":         in.Targeting,
		"status":            in.Status,
	}

	adsetID, err := mc.CreateAdSet(ctx, client.AdAccountID, payload)
	if err != nil {
		return CreateAdSetOutput{}, err
	}

	return CreateAdSetOutput{AdSetID: adsetID}, nil
}
