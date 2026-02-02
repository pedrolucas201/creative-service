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
	ClientID             string
	Name                 string
	Objective            string
	Status               string
	SpecialAdCategories  []string
}

type CreateCampaignOutput struct {
	CampaignID string `json:"campaign_id"`
}

func (s *CampaignService) CreateCampaign(ctx context.Context, in CreateCampaignInput) (CreateCampaignOutput, error) {
	if err := s.Sem.Acquire(ctx); err != nil {
		return CreateCampaignOutput{}, err
	}
	defer s.Sem.Release()

	client, err := s.Store.GetClient(ctx, in.ClientID)
	if err != nil {
		return CreateCampaignOutput{}, fmt.Errorf("get client: %w", err)
	}

	token, err := s.Tokens.Resolve(client.TokenRef)
	if err != nil {
		return CreateCampaignOutput{}, fmt.Errorf("resolve token: %w", err)
	}

	mc := meta.New(s.BaseURL, s.APIVersion, token, s.HTTPTimeout)

	payload := map[string]any{
		"name":      in.Name,
		"objective": in.Objective,
		"status":    in.Status,
	}

	if len(in.SpecialAdCategories) > 0 {
		payload["special_ad_categories"] = in.SpecialAdCategories
	}

	campaignID, err := mc.CreateCampaign(ctx, client.AdAccountID, payload)
	if err != nil {
		return CreateCampaignOutput{}, err
	}

	return CreateCampaignOutput{CampaignID: campaignID}, nil
}
