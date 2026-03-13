package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"creative-service/internal/bm"
	"creative-service/internal/meta"
	"creative-service/internal/secrets"
	"creative-service/internal/storage"
)

type AdService struct {
	Store  *storage.Store
	BM     *bm.Service
	Tokens secrets.Resolver

	BaseURL     string
	APIVersion  string
	HTTPTimeout time.Duration

	Sem *Semaphore
}

type CreateAdInput struct {
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

	bmCfg, err := s.BM.GetBMConfigByAdAccountID(ctx, adAccount.AdAccountID)
	if err != nil {
		return CreateAdOutput{}, fmt.Errorf("get bm config: %w", err)
	}

	token, err := s.Tokens.Resolve(bmCfg.TokenRef)
	if err != nil {
		return CreateAdOutput{}, fmt.Errorf("resolve token: %w", err)
	}

	mc := meta.New(s.BaseURL, s.APIVersion, token, s.HTTPTimeout)

	payload := map[string]any{
		"name":     in.Name,
		"adset_id": in.AdSetID,
		"creative": map[string]any{"creative_id": in.CreativeID},
		"status":   in.Status,
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
	ID               string         `json:"id"`
	Name             string         `json:"name"`
	CampaignID       string         `json:"campaign_id,omitempty"`
	AdSetID          string         `json:"adset_id,omitempty"`
	Status           string         `json:"status,omitempty"`
	ConfiguredStatus string         `json:"configured_status,omitempty"`
	EffectiveStatus  string         `json:"effective_status,omitempty"`
	Creative         map[string]any `json:"creative,omitempty"`
	CreatedTime      string         `json:"created_time,omitempty"`
}

type ListAdsOutput struct {
	Ads []AdItem `json:"ads"`
}

func adStatusRank(status string) int {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "DISAPPROVED":
		return 8
	case "WITH_ISSUES":
		return 7
	case "PENDING_REVIEW":
		return 6
	case "IN_PROCESS":
		return 5
	case "ACTIVE":
		return 4
	case "PAUSED":
		return 3
	case "PREAPPROVED":
		return 2
	case "ARCHIVED":
		return 1
	case "DELETED":
		return 0
	default:
		return 0
	}
}

func pickCreativeStatus(current, candidate string) string {
	if adStatusRank(candidate) > adStatusRank(current) {
		return candidate
	}
	return current
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

	bmCfg, err := s.BM.GetBMConfigByAdAccountID(ctx, adAccount.AdAccountID)
	if err != nil {
		return ListAdsOutput{}, fmt.Errorf("get bm config: %w", err)
	}

	token, err := s.Tokens.Resolve(bmCfg.TokenRef)
	if err != nil {
		return ListAdsOutput{}, fmt.Errorf("resolve token: %w", err)
	}

	mc := meta.New(s.BaseURL, s.APIVersion, token, s.HTTPTimeout)

	fields := []string{
		"id",
		"name",
		"campaign_id",
		"adset_id",
		"status",
		"configured_status",
		"effective_status",
		"creative{id,name}",
		"created_time",
	}
	data, err := mc.ListAds(ctx, adAccount.AdAccountID, fields)
	if err != nil {
		return ListAdsOutput{}, err
	}

	ads := make([]AdItem, 0, len(data))
	statusRows := make([]storage.EntityStatusUpsert, 0, len(data)*2)
	creativeBestStatus := make(map[string]string)
	creativeRawByID := make(map[string]map[string]any)

	for _, item := range data {
		a := AdItem{}
		if id, ok := item["id"].(string); ok {
			a.ID = id
		}
		if name, ok := item["name"].(string); ok {
			a.Name = name
		}
		if campaignID, ok := item["campaign_id"].(string); ok {
			a.CampaignID = campaignID
		}
		if asid, ok := item["adset_id"].(string); ok {
			a.AdSetID = asid
		}
		if status, ok := item["status"].(string); ok {
			a.Status = status
		}
		if configured, ok := item["configured_status"].(string); ok {
			a.ConfiguredStatus = configured
		}
		if effective, ok := item["effective_status"].(string); ok {
			a.EffectiveStatus = effective
		}
		if creative, ok := item["creative"].(map[string]any); ok {
			a.Creative = creative
		}
		if ct, ok := item["created_time"].(string); ok {
			a.CreatedTime = ct
		}
		a.Status = resolveEntityStatus(a.EffectiveStatus, a.ConfiguredStatus, a.Status)

		if a.ID != "" {
			rawAd, err := json.Marshal(item)
			if err != nil {
				return ListAdsOutput{}, fmt.Errorf("marshal ad payload: %w", err)
			}
			statusRows = append(statusRows, storage.EntityStatusUpsert{
				EntityType:  "ad",
				EntityID:    a.ID,
				AdAccountID: adAccount.AdAccountID,
				Status:      a.Status,
				RawPayload:  rawAd,
			})
		}

		if creativeID, ok := a.Creative["id"].(string); ok && creativeID != "" {
			creativeBestStatus[creativeID] = pickCreativeStatus(creativeBestStatus[creativeID], a.Status)
			creativeRawByID[creativeID] = map[string]any{
				"creative":                    a.Creative,
				"source_ad_id":                a.ID,
				"source_ad_status":            a.Status,
				"source_ad_effective_status":  a.EffectiveStatus,
				"source_ad_configured_status": a.ConfiguredStatus,
			}
		}

		ads = append(ads, a)
	}

	for creativeID, status := range creativeBestStatus {
		rawCreative, err := json.Marshal(creativeRawByID[creativeID])
		if err != nil {
			return ListAdsOutput{}, fmt.Errorf("marshal creative status payload: %w", err)
		}

		statusRows = append(statusRows, storage.EntityStatusUpsert{
			EntityType:  "creative",
			EntityID:    creativeID,
			AdAccountID: adAccount.AdAccountID,
			Status:      status,
			RawPayload:  rawCreative,
		})
	}

	if err := s.Store.UpsertEntityStatuses(ctx, statusRows); err != nil {
		return ListAdsOutput{}, fmt.Errorf("sync ad/creative status cache: %w", err)
	}

	return ListAdsOutput{Ads: ads}, nil
}

// ======= UPDATE Ad =======

type UpdateAdInput struct {
	AdAccountID string // necessário para resolver token
	AdID        string
	Name        *string // opcional
	Status      *string // opcional (ACTIVE, PAUSED, DELETED)
}

func (s *AdService) UpdateAd(ctx context.Context, in UpdateAdInput) error {
	if err := s.Sem.Acquire(ctx); err != nil {
		return err
	}
	defer s.Sem.Release()

	adAccount, err := s.Store.GetAdAccount(ctx, in.AdAccountID)
	if err != nil {
		return fmt.Errorf("get ad account: %w", err)
	}

	bmCfg, err := s.BM.GetBMConfigByAdAccountID(ctx, adAccount.AdAccountID)
	if err != nil {
		return fmt.Errorf("get bm config: %w", err)
	}

	token, err := s.Tokens.Resolve(bmCfg.TokenRef)
	if err != nil {
		return fmt.Errorf("resolve token: %w", err)
	}

	mc := meta.New(s.BaseURL, s.APIVersion, token, s.HTTPTimeout)

	payload := map[string]any{}
	if in.Name != nil {
		payload["name"] = *in.Name
	}
	if in.Status != nil {
		payload["status"] = *in.Status
	}

	if len(payload) == 0 {
		return fmt.Errorf("no fields to update")
	}

	return mc.UpdateAd(ctx, in.AdID, payload)
}

// ======= DELETE Ad (soft delete) =======

type DeleteAdInput struct {
	AdAccountID string // necessário para resolver token
	AdID        string
}

func (s *AdService) DeleteAd(ctx context.Context, in DeleteAdInput) error {
	if err := s.Sem.Acquire(ctx); err != nil {
		return err
	}
	defer s.Sem.Release()

	adAccount, err := s.Store.GetAdAccount(ctx, in.AdAccountID)
	if err != nil {
		return fmt.Errorf("get ad account: %w", err)
	}

	bmCfg, err := s.BM.GetBMConfigByAdAccountID(ctx, adAccount.AdAccountID)
	if err != nil {
		return fmt.Errorf("get bm config: %w", err)
	}

	token, err := s.Tokens.Resolve(bmCfg.TokenRef)
	if err != nil {
		return fmt.Errorf("resolve token: %w", err)
	}

	mc := meta.New(s.BaseURL, s.APIVersion, token, s.HTTPTimeout)

	return mc.SoftDeleteAd(ctx, in.AdID)
}
