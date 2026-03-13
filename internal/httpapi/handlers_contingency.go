package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"creative-service/internal/meta"
	"creative-service/internal/service"
	"creative-service/internal/storage"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

type monitorContingencyRequest struct {
	AdAccountID   string `json:"ad_account_id"`
	DryRun        bool   `json:"dry_run"`
	RefreshStatus *bool  `json:"refresh_status"`
	MaxCandidates int    `json:"max_candidates"`
	TriggerType   string `json:"trigger_type"`
}

type monitorContingencyItem struct {
	CampaignID    string    `json:"campaign_id"`
	AdID          string    `json:"ad_id"`
	AdStatus      string    `json:"ad_status"`
	ErrorCode     string    `json:"error_code,omitempty"`
	ErrorSummary  string    `json:"error_summary,omitempty"`
	ErrorMessage  string    `json:"error_message,omitempty"`
	SyncedAt      time.Time `json:"synced_at"`
	IncidentUUID  string    `json:"incident_uuid,omitempty"`
	IncidentState string    `json:"incident_status,omitempty"`
	Created       bool      `json:"created"`
}

type executeContingencyRequest struct {
	IncidentUUID string `json:"incident_uuid"`
	MaxAttempts  int    `json:"max_attempts"`
}

type closeContingencyIncidentRequest struct {
	Status       string `json:"status"`
	ReasonDetail string `json:"reason_detail"`
}

type upsertContingencyNodeRequest struct {
	AdAccountID   string `json:"ad_account_id"`
	NodeName      string `json:"node_name"`
	Priority      int    `json:"priority"`
	Weight        int    `json:"weight"`
	IsActive      *bool  `json:"is_active"`
	CooldownUntil string `json:"cooldown_until"`
}

type upsertContingencyRouteRequest struct {
	SourceAdAccountID string `json:"source_ad_account_id"`
	TargetNodeUUID    string `json:"target_node_uuid"`
	OrderIndex        int    `json:"order_index"`
	IsActive          *bool  `json:"is_active"`
}

type contingencySwitchResult struct {
	TargetCampaignID  string                  `json:"target_campaign_id"`
	SwitchMap         storage.EntitySwitchMap `json:"switch_map"`
	SourceAdSetIDs    []string                `json:"source_adset_ids"`
	TargetAdSetIDs    []string                `json:"target_adset_ids"`
	SourceAdIDs       []string                `json:"source_ad_ids"`
	TargetAdIDs       []string                `json:"target_ad_ids"`
	SourceCreativeIDs []string                `json:"source_creative_ids,omitempty"`
	TargetCreativeIDs []string                `json:"target_creative_ids,omitempty"`
}

func normalizeContingencyTriggerType(raw string) string {
	v := strings.ToLower(strings.TrimSpace(raw))
	if v == "" {
		return "polling"
	}
	return v
}

func contingencyTriggerTypeAllowed(raw string) bool {
	switch normalizeContingencyTriggerType(raw) {
	case "polling", "webhook", "manual":
		return true
	default:
		return false
	}
}

func contingencyReasonCodeForAdStatus(status string) string {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "DISAPPROVED":
		return "ad_disapproved"
	case "WITH_ISSUES":
		return "ad_with_issues"
	default:
		return "ad_status_critical"
	}
}

func parseContingencyLimit(raw int) int {
	if raw <= 0 {
		return 50
	}
	if raw > 200 {
		return 200
	}
	return raw
}

func parseContingencyMaxAttempts(raw int) int {
	if raw <= 0 {
		return 3
	}
	if raw > 10 {
		return 10
	}
	return raw
}

func refreshStatusEnabled(raw *bool) bool {
	if raw == nil {
		return true
	}
	return *raw
}

func parseContingencyCooldown(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func dedupeCandidatesByCampaign(items []storage.ContingencyCandidate) []storage.ContingencyCandidate {
	out := make([]storage.ContingencyCandidate, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		campaignID := strings.TrimSpace(item.CampaignID)
		if campaignID == "" {
			continue
		}
		if _, ok := seen[campaignID]; ok {
			continue
		}
		seen[campaignID] = struct{}{}
		out = append(out, item)
	}
	return out
}

func contingencyReasonDetailFromCandidate(c storage.ContingencyCandidate) string {
	parts := make([]string, 0, 3)
	if strings.TrimSpace(c.ErrorCode) != "" {
		parts = append(parts, "code="+strings.TrimSpace(c.ErrorCode))
	}
	if strings.TrimSpace(c.ErrorSummary) != "" {
		parts = append(parts, strings.TrimSpace(c.ErrorSummary))
	}
	if strings.TrimSpace(c.ErrorMessage) != "" {
		parts = append(parts, strings.TrimSpace(c.ErrorMessage))
	}
	return strings.TrimSpace(strings.Join(parts, " | "))
}

func buildContingencyEvidence(c storage.ContingencyCandidate) json.RawMessage {
	raw, err := json.Marshal(map[string]any{
		"source":        "entity_status_cache",
		"entity_type":   "ad",
		"ad_id":         c.AdID,
		"campaign_id":   c.CampaignID,
		"status":        c.AdStatus,
		"error_code":    c.ErrorCode,
		"error_summary": c.ErrorSummary,
		"error_message": c.ErrorMessage,
		"synced_at":     c.SyncedAt,
	})
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return raw
}

func asString(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return ""
	}
}

func asStringSlice(value any) []string {
	switch typed := value.(type) {
	case []string:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			item = strings.TrimSpace(item)
			if item != "" {
				out = append(out, item)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok {
				text = strings.TrimSpace(text)
				if text != "" {
					out = append(out, text)
				}
			}
		}
		return out
	default:
		return nil
	}
}

func normalizeSpecialAdCategories(categories []string) []string {
	if len(categories) == 0 {
		return []string{}
	}

	out := make([]string, 0, len(categories))
	for _, category := range categories {
		category = strings.ToUpper(strings.TrimSpace(category))
		if category == "" || category == "NONE" {
			continue
		}
		out = append(out, category)
	}
	if len(out) == 0 {
		return []string{}
	}
	return out
}

func cloneCampaignPayload(base map[string]any) map[string]any {
	if len(base) == 0 {
		return nil
	}
	payload := make(map[string]any, len(base))
	for key, value := range base {
		payload[key] = value
	}
	return payload
}

type contingencyCampaignPayloadVariant struct {
	Label            string
	Payload          map[string]any
	PauseAfterCreate bool
}

func createContingencyCampaignPayloadVariants(name, objective string, specialCategories []string) []contingencyCampaignPayloadVariant {
	base := map[string]any{
		"name":                            name,
		"objective":                       objective,
		"status":                          "PAUSED",
		"buying_type":                     "AUCTION",
		"is_adset_budget_sharing_enabled": false,
	}

	categoryVariants := []struct {
		Label string
		Value any
	}{
		{
			Label: "special_categories_array",
			Value: specialCategories,
		},
	}

	if len(specialCategories) == 0 {
		categoryVariants = append(categoryVariants, struct {
			Label string
			Value any
		}{
			Label: "special_categories_none_scalar",
			Value: "NONE",
		})
	}

	out := make([]contingencyCampaignPayloadVariant, 0, len(categoryVariants)*4)
	for _, categoryVariant := range categoryVariants {
		full := cloneCampaignPayload(base)
		full["special_ad_categories"] = categoryVariant.Value
		out = append(out, contingencyCampaignPayloadVariant{
			Label:            "full_payload_" + categoryVariant.Label,
			Payload:          full,
			PauseAfterCreate: false,
		})

		withoutStatus := cloneCampaignPayload(full)
		delete(withoutStatus, "status")
		out = append(out, contingencyCampaignPayloadVariant{
			Label:            "without_status_" + categoryVariant.Label,
			Payload:          withoutStatus,
			PauseAfterCreate: true,
		})

		withoutBuyingType := cloneCampaignPayload(full)
		delete(withoutBuyingType, "buying_type")
		out = append(out, contingencyCampaignPayloadVariant{
			Label:            "without_buying_type_" + categoryVariant.Label,
			Payload:          withoutBuyingType,
			PauseAfterCreate: false,
		})

		withoutBuyingTypeAndStatus := cloneCampaignPayload(withoutBuyingType)
		delete(withoutBuyingTypeAndStatus, "status")
		out = append(out, contingencyCampaignPayloadVariant{
			Label:            "without_buying_type_without_status_" + categoryVariant.Label,
			Payload:          withoutBuyingTypeAndStatus,
			PauseAfterCreate: true,
		})
	}

	return out
}

func asMap(value any) map[string]any {
	typed, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	return typed
}

func deepCopyMap(value map[string]any) map[string]any {
	if len(value) == 0 {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}

func copyMapField(dst map[string]any, src map[string]any, key string) {
	if dst == nil || src == nil {
		return
	}
	value, ok := src[key]
	if !ok || value == nil {
		return
	}
	if nested := asMap(value); len(nested) > 0 {
		if cloned := deepCopyMap(nested); len(cloned) > 0 {
			dst[key] = cloned
		}
		return
	}
	if text := asString(value); text != "" {
		dst[key] = text
		return
	}
	dst[key] = value
}

func buildTargetCreativePayload(
	sourceCreative map[string]any,
	targetPageID string,
	suffix string,
) map[string]any {
	if len(sourceCreative) == 0 {
		return nil
	}

	payload := make(map[string]any)

	allowedMapKeys := []string{
		"object_story_spec",
		"asset_feed_spec",
		"degrees_of_freedom_spec",
		"template_data",
	}
	for _, key := range allowedMapKeys {
		copyMapField(payload, sourceCreative, key)
	}

	allowedScalarKeys := []string{
		"url_tags",
		"applink_treatment",
		"instagram_actor_id",
		"authorization_category",
		"object_type",
		"object_id",
		"product_set_id",
		"image_hash",
		"image_url",
		"video_id",
		"title",
		"body",
	}
	for _, key := range allowedScalarKeys {
		copyMapField(payload, sourceCreative, key)
	}

	if storySpec := asMap(payload["object_story_spec"]); len(storySpec) > 0 {
		updatedSpec := deepCopyMap(storySpec)
		if len(updatedSpec) > 0 {
			if targetPageID != "" {
				updatedSpec["page_id"] = targetPageID
			}
			payload["object_story_spec"] = updatedSpec
		}
	}

	if _, hasStorySpec := payload["object_story_spec"]; !hasStorySpec {
		if storyID := asString(sourceCreative["object_story_id"]); storyID != "" {
			payload["object_story_id"] = storyID
		}
	}

	name := asString(sourceCreative["name"])
	if name == "" {
		name = "Creative contingencia"
	}
	if suffix != "" {
		name = fmt.Sprintf("%s [%s]", name, suffix)
	}
	payload["name"] = name

	return payload
}

func asInt(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float32:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil {
			return 0
		}
		return parsed
	default:
		return 0
	}
}

func dedupeStringSlice(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for _, raw := range in {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func (h *Handler) contingencyMetaClient(ctx context.Context, adAccountID string) (*meta.Client, error) {
	adAccountID = strings.TrimSpace(adAccountID)
	if adAccountID == "" {
		return nil, errors.New("ad_account_id is required")
	}

	account, err := h.Store.GetAdAccount(ctx, adAccountID)
	if err != nil {
		return nil, fmt.Errorf("get ad account: %w", err)
	}

	bmCfg, err := h.BM.GetBMConfigByAdAccountID(ctx, account.AdAccountID)
	if err != nil {
		return nil, fmt.Errorf("get bm config: %w", err)
	}

	token, err := h.Campaigns.Tokens.Resolve(bmCfg.TokenRef)
	if err != nil {
		return nil, fmt.Errorf("resolve token: %w", err)
	}

	return meta.New(h.Campaigns.BaseURL, h.Campaigns.APIVersion, token, h.Campaigns.HTTPTimeout), nil
}

func (h *Handler) runContingencySwitch(
	ctx context.Context,
	incident storage.ContingencyIncident,
	targetNode storage.ContingencyNode,
) (contingencySwitchResult, error) {
	sourceMeta, err := h.contingencyMetaClient(ctx, incident.SourceAdAccountID)
	if err != nil {
		return contingencySwitchResult{}, err
	}

	targetMeta, err := h.contingencyMetaClient(ctx, targetNode.AdAccountID)
	if err != nil {
		return contingencySwitchResult{}, err
	}

	targetBMCfg, err := h.BM.GetBMConfigByAdAccountID(ctx, targetNode.AdAccountID)
	if err != nil {
		return contingencySwitchResult{}, fmt.Errorf("get target bm config: %w", err)
	}
	targetPageID := strings.TrimSpace(targetBMCfg.PageID)

	sourceCampaignRaw, err := sourceMeta.GetObject(ctx, incident.SourceCampaignID, []string{
		"id",
		"name",
		"objective",
		"special_ad_categories",
		"buying_type",
	})
	if err != nil {
		return contingencySwitchResult{}, fmt.Errorf("get source campaign: %w", err)
	}

	campaignName := asString(sourceCampaignRaw["name"])
	if campaignName == "" {
		campaignName = incident.SourceCampaignID
	}

	targetCampaignName := fmt.Sprintf("%s [CONTINGENCIA %s]", campaignName, time.Now().Format("2006-01-02 15:04"))
	targetCampaignObjective := asString(sourceCampaignRaw["objective"])
	if targetCampaignObjective == "" {
		return contingencySwitchResult{}, errors.New("source campaign missing objective")
	}

	specialCategories := normalizeSpecialAdCategories(asStringSlice(sourceCampaignRaw["special_ad_categories"]))
	targetCampaignPayloads := createContingencyCampaignPayloadVariants(
		targetCampaignName,
		targetCampaignObjective,
		specialCategories,
	)

	var targetCampaignID string
	var createCampaignErr error
	var successfulVariant string
	var attemptedVariants []string
	for idx, variant := range targetCampaignPayloads {
		attemptedVariants = append(attemptedVariants, variant.Label)
		targetCampaignID, createCampaignErr = targetMeta.CreateCampaign(ctx, targetNode.AdAccountID, variant.Payload)
		if createCampaignErr == nil {
			successfulVariant = variant.Label
			if variant.PauseAfterCreate {
				if err := targetMeta.UpdateCampaign(ctx, targetCampaignID, map[string]any{"status": "PAUSED"}); err != nil {
					return contingencySwitchResult{}, fmt.Errorf("pause target campaign after create: %w", err)
				}
			}
			break
		}
		if !meta.IsInvalidParameterError(createCampaignErr) || idx == len(targetCampaignPayloads)-1 {
			break
		}
	}
	if createCampaignErr != nil {
		return contingencySwitchResult{}, fmt.Errorf(
			"create target campaign (objective=%s special_ad_categories=%v variants=%v): %w",
			targetCampaignObjective,
			specialCategories,
			attemptedVariants,
			createCampaignErr,
		)
	}
	_ = successfulVariant

	adsetRows, err := sourceMeta.ListAdSets(ctx, incident.SourceAdAccountID, []string{
		"id",
		"name",
		"campaign_id",
		"billing_event",
		"optimization_goal",
		"bid_amount",
		"bid_strategy",
		"daily_budget",
		"lifetime_budget",
		"targeting",
		"promoted_object",
		"status",
	})
	if err != nil {
		return contingencySwitchResult{}, fmt.Errorf("list source adsets: %w", err)
	}

	sourceAdSetIDs := make([]string, 0)
	targetAdSetIDs := make([]string, 0)
	adsetMap := make(map[string]string)

	for _, adset := range adsetRows {
		if asString(adset["campaign_id"]) != incident.SourceCampaignID {
			continue
		}

		sourceAdSetID := asString(adset["id"])
		if sourceAdSetID == "" {
			continue
		}

		billingEvent := asString(adset["billing_event"])
		optimizationGoal := asString(adset["optimization_goal"])
		if billingEvent == "" || optimizationGoal == "" {
			return contingencySwitchResult{}, fmt.Errorf("source adset %s missing required fields", sourceAdSetID)
		}

		targeting := asMap(adset["targeting"])
		if len(targeting) == 0 {
			return contingencySwitchResult{}, fmt.Errorf("source adset %s missing targeting", sourceAdSetID)
		}

		payload := map[string]any{
			"campaign_id":       targetCampaignID,
			"name":              asString(adset["name"]),
			"billing_event":     billingEvent,
			"optimization_goal": optimizationGoal,
			"targeting":         targeting,
			"status":            "PAUSED",
		}

		if payload["name"] == "" {
			payload["name"] = "AdSet contingencia " + sourceAdSetID
		}

		if bidStrategy := asString(adset["bid_strategy"]); bidStrategy != "" {
			payload["bid_strategy"] = bidStrategy
		}
		if bidAmount := asInt(adset["bid_amount"]); bidAmount > 0 {
			payload["bid_amount"] = bidAmount
		}

		if dailyBudget := asInt(adset["daily_budget"]); dailyBudget > 0 {
			payload["daily_budget"] = dailyBudget
		} else if lifetimeBudget := asInt(adset["lifetime_budget"]); lifetimeBudget > 0 {
			payload["lifetime_budget"] = lifetimeBudget
		} else {
			return contingencySwitchResult{}, fmt.Errorf("source adset %s missing budget", sourceAdSetID)
		}

		if promotedObject := asMap(adset["promoted_object"]); len(promotedObject) > 0 {
			payload["promoted_object"] = promotedObject
		}

		targetAdSetID, createErr := targetMeta.CreateAdSet(ctx, targetNode.AdAccountID, payload)
		if createErr != nil {
			return contingencySwitchResult{}, fmt.Errorf("create target adset from %s: %w", sourceAdSetID, createErr)
		}

		adsetMap[sourceAdSetID] = targetAdSetID
		sourceAdSetIDs = append(sourceAdSetIDs, sourceAdSetID)
		targetAdSetIDs = append(targetAdSetIDs, targetAdSetID)
	}

	if len(adsetMap) == 0 {
		return contingencySwitchResult{}, errors.New("no source adset found for source campaign")
	}

	adRows, err := sourceMeta.ListAds(ctx, incident.SourceAdAccountID, []string{
		"id",
		"name",
		"campaign_id",
		"adset_id",
		"creative{id}",
		"status",
	})
	if err != nil {
		return contingencySwitchResult{}, fmt.Errorf("list source ads: %w", err)
	}

	sourceAdIDs := make([]string, 0)
	targetAdIDs := make([]string, 0)
	sourceCreativeIDs := make([]string, 0)
	targetCreativeIDs := make([]string, 0)
	creativeMap := make(map[string]string)
	adsCopied := 0

	for _, ad := range adRows {
		if asString(ad["campaign_id"]) != incident.SourceCampaignID {
			continue
		}

		sourceAdID := asString(ad["id"])
		sourceAdSetID := asString(ad["adset_id"])
		targetAdSetID, ok := adsetMap[sourceAdSetID]
		if sourceAdID == "" || !ok || targetAdSetID == "" {
			continue
		}

		creative := asMap(ad["creative"])
		sourceCreativeID := asString(creative["id"])
		if sourceCreativeID == "" {
			return contingencySwitchResult{}, fmt.Errorf("source ad %s missing creative.id", sourceAdID)
		}

		targetCreativeID, alreadyCloned := creativeMap[sourceCreativeID]
		if !alreadyCloned {
			sourceCreative, getCreativeErr := sourceMeta.GetCreative(ctx, sourceCreativeID, []string{
				"id",
				"name",
				"object_story_spec",
				"asset_feed_spec",
				"degrees_of_freedom_spec",
				"template_data",
				"url_tags",
				"applink_treatment",
				"instagram_actor_id",
				"authorization_category",
				"object_type",
				"object_id",
				"product_set_id",
				"image_hash",
				"image_url",
				"video_id",
				"title",
				"body",
				"object_story_id",
			})
			if getCreativeErr != nil {
				return contingencySwitchResult{}, fmt.Errorf("get source creative %s: %w", sourceCreativeID, getCreativeErr)
			}

			nameSuffix := "CONTINGENCIA " + time.Now().Format("2006-01-02 15:04")
			targetCreativePayload := buildTargetCreativePayload(sourceCreative, targetPageID, nameSuffix)
			if len(targetCreativePayload) <= 1 {
				return contingencySwitchResult{}, fmt.Errorf("source creative %s has no reusable payload", sourceCreativeID)
			}

			targetCreativeID, err = targetMeta.CreateCreative(ctx, targetNode.AdAccountID, targetCreativePayload)
			if err != nil {
				return contingencySwitchResult{}, fmt.Errorf("create target creative from %s: %w", sourceCreativeID, err)
			}

			creativeMap[sourceCreativeID] = targetCreativeID
			sourceCreativeIDs = append(sourceCreativeIDs, sourceCreativeID)
			targetCreativeIDs = append(targetCreativeIDs, targetCreativeID)
		}

		payload := map[string]any{
			"name":     asString(ad["name"]),
			"adset_id": targetAdSetID,
			"creative": map[string]any{"creative_id": targetCreativeID},
			"status":   "PAUSED",
		}
		if payload["name"] == "" {
			payload["name"] = "Ad contingencia " + sourceAdID
		}

		targetAdID, createErr := targetMeta.CreateAd(ctx, targetNode.AdAccountID, payload)
		if createErr != nil {
			return contingencySwitchResult{}, fmt.Errorf("create target ad from %s: %w", sourceAdID, createErr)
		}

		sourceAdIDs = append(sourceAdIDs, sourceAdID)
		targetAdIDs = append(targetAdIDs, targetAdID)
		adsCopied++
	}

	if adsCopied == 0 {
		return contingencySwitchResult{}, errors.New("no source ad copied for source campaign")
	}

	if err := sourceMeta.UpdateCampaign(ctx, incident.SourceCampaignID, map[string]any{"status": "PAUSED"}); err != nil {
		return contingencySwitchResult{}, fmt.Errorf("pause source campaign: %w", err)
	}

	switchMap, err := h.Store.CreateEntitySwitchMap(ctx, storage.CreateEntitySwitchMapInput{
		IncidentUUID:     incident.IncidentUUID,
		SourceCampaignID: incident.SourceCampaignID,
		TargetCampaignID: targetCampaignID,
		SourceAdSetIDs:   dedupeStringSlice(sourceAdSetIDs),
		TargetAdSetIDs:   dedupeStringSlice(targetAdSetIDs),
		SourceAdIDs:      dedupeStringSlice(sourceAdIDs),
		TargetAdIDs:      dedupeStringSlice(targetAdIDs),
	})
	if err != nil {
		return contingencySwitchResult{}, fmt.Errorf("create switch map: %w", err)
	}

	return contingencySwitchResult{
		TargetCampaignID:  targetCampaignID,
		SwitchMap:         switchMap,
		SourceAdSetIDs:    dedupeStringSlice(sourceAdSetIDs),
		TargetAdSetIDs:    dedupeStringSlice(targetAdSetIDs),
		SourceAdIDs:       dedupeStringSlice(sourceAdIDs),
		TargetAdIDs:       dedupeStringSlice(targetAdIDs),
		SourceCreativeIDs: dedupeStringSlice(sourceCreativeIDs),
		TargetCreativeIDs: dedupeStringSlice(targetCreativeIDs),
	}, nil
}

type monitorContingencyRunResult struct {
	CandidatesScanned int                      `json:"candidates_scanned"`
	CandidatesDeduped int                      `json:"candidates_deduped"`
	IncidentsCreated  int                      `json:"incidents_created"`
	IncidentsExisting int                      `json:"incidents_existing"`
	IncidentsTotal    int                      `json:"incidents_processed"`
	Items             []monitorContingencyItem `json:"items"`
	Failed            []string                 `json:"failed,omitempty"`
}

func (h *Handler) runMonitorContingency(
	ctx context.Context,
	req monitorContingencyRequest,
) (monitorContingencyRunResult, error) {
	doRefresh := refreshStatusEnabled(req.RefreshStatus)
	if doRefresh {
		if _, err := h.Ads.ListAds(ctx, service.ListAdsInput{
			AdAccountID: req.AdAccountID,
		}); err != nil {
			return monitorContingencyRunResult{}, fmt.Errorf("sync statuses: %w", err)
		}
	}

	candidates, err := h.Store.ListContingencyCandidatesByAdAccount(
		ctx,
		req.AdAccountID,
		req.MaxCandidates,
	)
	if err != nil {
		return monitorContingencyRunResult{}, err
	}

	deduped := dedupeCandidatesByCampaign(candidates)
	items := make([]monitorContingencyItem, 0, len(deduped))
	failed := make([]string, 0)
	createdCount := 0
	existingCount := 0

	for _, candidate := range deduped {
		item := monitorContingencyItem{
			CampaignID:   candidate.CampaignID,
			AdID:         candidate.AdID,
			AdStatus:     candidate.AdStatus,
			ErrorCode:    candidate.ErrorCode,
			ErrorSummary: candidate.ErrorSummary,
			ErrorMessage: candidate.ErrorMessage,
			SyncedAt:     candidate.SyncedAt,
		}

		if req.DryRun {
			items = append(items, item)
			continue
		}

		incident, created, incidentErr := h.Store.CreateOrGetOpenContingencyIncident(
			ctx,
			storage.CreateContingencyIncidentInput{
				SourceCampaignID:  candidate.CampaignID,
				SourceAdAccountID: req.AdAccountID,
				TriggerType:       req.TriggerType,
				ReasonCode:        contingencyReasonCodeForAdStatus(candidate.AdStatus),
				ReasonDetail:      contingencyReasonDetailFromCandidate(candidate),
				Evidence:          buildContingencyEvidence(candidate),
			},
		)
		if incidentErr != nil {
			failed = append(failed, fmt.Sprintf("campaign %s: %v", candidate.CampaignID, incidentErr))
			continue
		}

		item.IncidentUUID = incident.IncidentUUID
		item.IncidentState = incident.Status
		item.Created = created
		if created {
			createdCount++
		} else {
			existingCount++
		}

		items = append(items, item)
	}

	return monitorContingencyRunResult{
		CandidatesScanned: len(candidates),
		CandidatesDeduped: len(deduped),
		IncidentsCreated:  createdCount,
		IncidentsExisting: existingCount,
		IncidentsTotal:    len(items),
		Items:             items,
		Failed:            failed,
	}, nil
}

func (h *Handler) GetContingencyConfig(w http.ResponseWriter, r *http.Request) {
	adAccountID := strings.TrimSpace(r.URL.Query().Get("ad_account_id"))
	if adAccountID == "" {
		writeErr(w, http.StatusBadRequest, "missing_ad_account_id")
		return
	}
	if !h.requireAdAccountAccessWithRoles(w, r, adAccountID, contingencyManualRoles) {
		return
	}

	sourceAccount, err := h.Store.GetAdAccount(r.Context(), adAccountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "not_found")
			return
		}
		writeErrCause(w, http.StatusInternalServerError, "failed_to_get_contingency_source_account", err)
		return
	}

	availableAccounts, err := h.Store.ListContingencyAvailableAccountsBySource(r.Context(), adAccountID)
	if err != nil {
		writeErrCause(w, http.StatusInternalServerError, "failed_to_list_contingency_available_accounts", err)
		return
	}

	nodes, err := h.Store.ListContingencyNodesBySource(r.Context(), adAccountID)
	if err != nil {
		writeErrCause(w, http.StatusInternalServerError, "failed_to_list_contingency_nodes", err)
		return
	}

	routes, err := h.Store.ListContingencyRoutesBySource(r.Context(), adAccountID)
	if err != nil {
		writeErrCause(w, http.StatusInternalServerError, "failed_to_list_contingency_routes", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"source_account":     sourceAccount,
		"available_accounts": availableAccounts,
		"available_count":    len(availableAccounts),
		"nodes":              nodes,
		"node_count":         len(nodes),
		"routes":             routes,
		"route_count":        len(routes),
	})
}

func (h *Handler) UpsertContingencyNode(w http.ResponseWriter, r *http.Request) {
	var req upsertContingencyNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		writeErr(w, http.StatusBadRequest, "invalid_json")
		return
	}

	req.AdAccountID = strings.TrimSpace(req.AdAccountID)
	req.NodeName = strings.TrimSpace(req.NodeName)
	if req.AdAccountID == "" {
		writeErr(w, http.StatusBadRequest, "missing_ad_account_id")
		return
	}
	if req.NodeName == "" {
		writeErr(w, http.StatusBadRequest, "missing_name")
		return
	}
	if !h.requireAdAccountAccessWithRoles(w, r, req.AdAccountID, contingencyManualRoles) {
		return
	}

	cooldownUntil, err := parseContingencyCooldown(req.CooldownUntil)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_contingency_cooldown")
		return
	}

	node, err := h.Store.UpsertContingencyNode(r.Context(), storage.UpsertContingencyNodeInput{
		AdAccountID:   req.AdAccountID,
		NodeName:      req.NodeName,
		Priority:      req.Priority,
		Weight:        req.Weight,
		IsActive:      req.IsActive == nil || *req.IsActive,
		CooldownUntil: cooldownUntil,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErr(w, http.StatusBadRequest, "invalid_contingency_node")
			return
		}
		writeErrCause(w, http.StatusInternalServerError, "failed_to_upsert_contingency_node", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"node":          node,
		"ad_account_id": node.AdAccountID,
		"user_message":  "Nó de contingência salvo com sucesso.",
	})
}

func (h *Handler) UpsertContingencyRoute(w http.ResponseWriter, r *http.Request) {
	var req upsertContingencyRouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		writeErr(w, http.StatusBadRequest, "invalid_json")
		return
	}

	req.SourceAdAccountID = strings.TrimSpace(req.SourceAdAccountID)
	req.TargetNodeUUID = strings.TrimSpace(req.TargetNodeUUID)
	if req.SourceAdAccountID == "" {
		writeErr(w, http.StatusBadRequest, "missing_ad_account_id")
		return
	}
	if req.TargetNodeUUID == "" {
		writeErr(w, http.StatusBadRequest, "missing_contingency_target_node_uuid")
		return
	}
	if !h.requireAdAccountAccessWithRoles(w, r, req.SourceAdAccountID, contingencyManualRoles) {
		return
	}

	route, err := h.Store.UpsertContingencyRoute(r.Context(), storage.UpsertContingencyRouteInput{
		SourceAdAccountID: req.SourceAdAccountID,
		TargetNodeUUID:    req.TargetNodeUUID,
		OrderIndex:        req.OrderIndex,
		IsActive:          req.IsActive == nil || *req.IsActive,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErr(w, http.StatusBadRequest, "invalid_contingency_route")
			return
		}
		writeErrCause(w, http.StatusInternalServerError, "failed_to_upsert_contingency_route", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"route":        route,
		"user_message": "Rota de contingência salva com sucesso.",
	})
}

func (h *Handler) DeleteContingencyRoute(w http.ResponseWriter, r *http.Request) {
	routeUUID := strings.TrimSpace(chi.URLParam(r, "route_uuid"))
	if routeUUID == "" {
		writeErr(w, http.StatusBadRequest, "missing_contingency_route_uuid")
		return
	}

	route, err := h.Store.GetContingencyRouteByUUID(r.Context(), routeUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "contingency_route_not_found")
			return
		}
		writeErrCause(w, http.StatusInternalServerError, "failed_to_get_contingency_route", err)
		return
	}

	if !h.requireAdAccountAccessWithRoles(w, r, route.SourceAdAccountID, contingencyManualRoles) {
		return
	}

	deleted, err := h.Store.DeleteContingencyRoute(r.Context(), routeUUID)
	if err != nil {
		writeErrCause(w, http.StatusInternalServerError, "failed_to_delete_contingency_route", err)
		return
	}
	if !deleted {
		writeErr(w, http.StatusNotFound, "contingency_route_not_found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"route_uuid":        routeUUID,
		"deleted":           true,
		"user_message":      "Rota de contingência removida com sucesso.",
		"source_ad_account": route.SourceAdAccountID,
	})
}

func (h *Handler) MonitorContingency(w http.ResponseWriter, r *http.Request) {
	var req monitorContingencyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		writeErr(w, http.StatusBadRequest, "invalid_json")
		return
	}

	req.AdAccountID = strings.TrimSpace(req.AdAccountID)
	if req.AdAccountID == "" {
		writeErr(w, http.StatusBadRequest, "missing_ad_account_id")
		return
	}
	if !h.requireAdAccountWriteAccess(w, r, req.AdAccountID) {
		return
	}

	req.TriggerType = normalizeContingencyTriggerType(req.TriggerType)
	if !contingencyTriggerTypeAllowed(req.TriggerType) {
		writeErr(w, http.StatusBadRequest, "invalid_contingency_trigger_type")
		return
	}

	req.MaxCandidates = parseContingencyLimit(req.MaxCandidates)
	result, err := h.runMonitorContingency(r.Context(), req)
	if err != nil {
		errMsg := strings.ToLower(strings.TrimSpace(err.Error()))
		if strings.Contains(errMsg, "sync statuses") {
			writeErrCause(w, http.StatusInternalServerError, "failed_to_sync_statuses", err)
			return
		}
		writeErrCause(w, http.StatusInternalServerError, "failed_to_list_contingency_candidates", err)
		return
	}

	resp := map[string]any{
		"ad_account_id":       req.AdAccountID,
		"dry_run":             req.DryRun,
		"refresh_status":      refreshStatusEnabled(req.RefreshStatus),
		"trigger_type":        req.TriggerType,
		"candidates_scanned":  result.CandidatesScanned,
		"candidates_deduped":  result.CandidatesDeduped,
		"incidents_created":   result.IncidentsCreated,
		"incidents_existing":  result.IncidentsExisting,
		"incidents_processed": result.IncidentsTotal,
		"items":               result.Items,
	}
	if len(result.Failed) > 0 {
		resp["failed"] = result.Failed
	}

	statusCode := http.StatusOK
	if len(result.Failed) > 0 {
		statusCode = http.StatusMultiStatus
	}
	writeJSON(w, statusCode, resp)
}

func (h *Handler) ListContingencyIncidents(w http.ResponseWriter, r *http.Request) {
	adAccountID := strings.TrimSpace(r.URL.Query().Get("ad_account_id"))
	if adAccountID == "" {
		writeErr(w, http.StatusBadRequest, "missing_ad_account_id")
		return
	}
	if !h.requireAdAccountAccess(w, r, adAccountID) {
		return
	}

	statusFilter := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	onlyOpen := true
	switch statusFilter {
	case "", "open":
		onlyOpen = true
	case "all":
		onlyOpen = false
	default:
		writeErr(w, http.StatusBadRequest, "invalid_contingency_status_filter")
		return
	}

	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "invalid_limit")
			return
		}
		limit = parseContingencyLimit(parsed)
	}

	items, err := h.Store.ListContingencyIncidentsByAdAccount(r.Context(), adAccountID, onlyOpen, limit)
	if err != nil {
		writeErrCause(w, http.StatusInternalServerError, "failed_to_list_contingency_incidents", err)
		return
	}

	outStatus := "open"
	if !onlyOpen {
		outStatus = "all"
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ad_account_id": adAccountID,
		"status":        outStatus,
		"count":         len(items),
		"incidents":     items,
	})
}

func contingencyIncidentStatusAfterFailure(attempt, maxAttempts int) string {
	if attempt >= maxAttempts {
		return "manual_required"
	}
	return "failed"
}

func parseContingencyCloseStatus(raw string) (string, bool) {
	v := strings.ToLower(strings.TrimSpace(raw))
	if v == "" {
		return "closed", true
	}
	switch v {
	case "closed", "manual_required":
		return v, true
	default:
		return "", false
	}
}

func (h *Handler) loadContingencyIncident(
	w http.ResponseWriter,
	r *http.Request,
	incidentUUID string,
) (storage.ContingencyIncident, bool) {
	incident, err := h.Store.GetContingencyIncidentByUUID(r.Context(), incidentUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "contingency_incident_not_found")
			return storage.ContingencyIncident{}, false
		}
		writeErrCause(w, http.StatusInternalServerError, "failed_to_get_contingency_incident", err)
		return storage.ContingencyIncident{}, false
	}
	return incident, true
}

func (h *Handler) executeContingencyIncident(
	w http.ResponseWriter,
	r *http.Request,
	incident storage.ContingencyIncident,
	maxAttempts int,
) {
	updatedIncident, execution, err := h.Store.StartContingencyExecution(r.Context(), incident.IncidentUUID)
	if err != nil {
		msg := strings.ToLower(strings.TrimSpace(err.Error()))
		switch {
		case strings.Contains(msg, "already in progress"):
			writeErr(w, http.StatusConflict, "contingency_execution_in_progress")
			return
		case strings.Contains(msg, "not executable"):
			writeErr(w, http.StatusConflict, "contingency_incident_not_executable")
			return
		default:
			writeErrCause(w, http.StatusInternalServerError, "failed_to_start_contingency_execution", err)
			return
		}
	}

	targetNode, found, err := h.Store.PickContingencyTargetNode(r.Context(), updatedIncident.SourceAdAccountID)
	if err != nil {
		failedIncidentStatus := contingencyIncidentStatusAfterFailure(updatedIncident.AttemptCount, maxAttempts)
		_, _, _ = h.Store.CompleteContingencyExecution(r.Context(), storage.CompleteContingencyExecutionInput{
			IncidentUUID:    updatedIncident.IncidentUUID,
			ExecutionUUID:   execution.ExecutionUUID,
			ExecutionStatus: "failed",
			IncidentStatus:  failedIncidentStatus,
			ErrorCode:       "failed_to_pick_contingency_target_node",
			ErrorMessage:    err.Error(),
		})
		writeErrCause(w, http.StatusInternalServerError, "failed_to_pick_contingency_target_node", err)
		return
	}

	if !found {
		failedIncidentStatus := contingencyIncidentStatusAfterFailure(updatedIncident.AttemptCount, maxAttempts)
		afterFailureIncident, afterFailureExecution, completeErr := h.Store.CompleteContingencyExecution(r.Context(), storage.CompleteContingencyExecutionInput{
			IncidentUUID:    updatedIncident.IncidentUUID,
			ExecutionUUID:   execution.ExecutionUUID,
			ExecutionStatus: "failed",
			IncidentStatus:  failedIncidentStatus,
			ErrorCode:       "no_target_node_for_contingency",
			ErrorMessage:    "no eligible contingency node found in same bm",
		})
		if completeErr != nil {
			writeErrCause(w, http.StatusInternalServerError, "failed_to_complete_contingency_execution", completeErr)
			return
		}

		writeJSON(w, http.StatusConflict, map[string]any{
			"incident_uuid":      afterFailureIncident.IncidentUUID,
			"incident_status":    afterFailureIncident.Status,
			"attempt_count":      afterFailureIncident.AttemptCount,
			"execution_uuid":     afterFailureExecution.ExecutionUUID,
			"execution_status":   afterFailureExecution.Status,
			"max_attempts":       maxAttempts,
			"next_step":          "configure_contingency_nodes_or_routes",
			"target_node_found":  false,
			"user_message":       "Nenhum nó de contingência elegível foi encontrado para esta conta.",
			"source_ad_account":  updatedIncident.SourceAdAccountID,
			"source_campaign_id": updatedIncident.SourceCampaignID,
		})
		return
	}

	switchResult, switchErr := h.runContingencySwitch(r.Context(), updatedIncident, targetNode)
	if switchErr != nil {
		failedIncidentStatus := contingencyIncidentStatusAfterFailure(updatedIncident.AttemptCount, maxAttempts)
		afterFailureIncident, afterFailureExecution, completeErr := h.Store.CompleteContingencyExecution(r.Context(), storage.CompleteContingencyExecutionInput{
			IncidentUUID:    updatedIncident.IncidentUUID,
			ExecutionUUID:   execution.ExecutionUUID,
			ExecutionStatus: "failed",
			IncidentStatus:  failedIncidentStatus,
			ErrorCode:       "contingency_switch_failed",
			ErrorMessage:    switchErr.Error(),
		})
		if completeErr != nil {
			writeErrCause(w, http.StatusInternalServerError, "failed_to_complete_contingency_execution", completeErr)
			return
		}

		writeJSON(w, http.StatusConflict, map[string]any{
			"incident_uuid":      afterFailureIncident.IncidentUUID,
			"incident_status":    afterFailureIncident.Status,
			"attempt_count":      afterFailureIncident.AttemptCount,
			"execution_uuid":     afterFailureExecution.ExecutionUUID,
			"execution_status":   afterFailureExecution.Status,
			"max_attempts":       maxAttempts,
			"next_step":          "check_switch_error_and_retry",
			"target_node_found":  true,
			"target_node":        targetNode,
			"user_message":       "Falha ao executar o switch da campanha para a conta de contingência.",
			"source_ad_account":  updatedIncident.SourceAdAccountID,
			"source_campaign_id": updatedIncident.SourceCampaignID,
			"error_code":         "contingency_switch_failed",
			"error_detail":       switchErr.Error(),
		})
		return
	}

	targetNodeUUID := targetNode.NodeUUID
	doneIncident, doneExecution, err := h.Store.CompleteContingencyExecution(r.Context(), storage.CompleteContingencyExecutionInput{
		IncidentUUID:    updatedIncident.IncidentUUID,
		ExecutionUUID:   execution.ExecutionUUID,
		ExecutionStatus: "succeeded",
		IncidentStatus:  "switched",
		TargetNodeUUID:  &targetNodeUUID,
	})
	if err != nil {
		writeErrCause(w, http.StatusInternalServerError, "failed_to_complete_contingency_execution", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"incident_uuid":       doneIncident.IncidentUUID,
		"incident_status":     doneIncident.Status,
		"attempt_count":       doneIncident.AttemptCount,
		"execution_uuid":      doneExecution.ExecutionUUID,
		"execution_status":    doneExecution.Status,
		"source_ad_account":   doneIncident.SourceAdAccountID,
		"source_campaign_id":  doneIncident.SourceCampaignID,
		"target_node_found":   true,
		"target_node":         targetNode,
		"target_campaign_id":  switchResult.TargetCampaignID,
		"source_adset_ids":    switchResult.SourceAdSetIDs,
		"target_adset_ids":    switchResult.TargetAdSetIDs,
		"source_ad_ids":       switchResult.SourceAdIDs,
		"target_ad_ids":       switchResult.TargetAdIDs,
		"source_creative_ids": switchResult.SourceCreativeIDs,
		"target_creative_ids": switchResult.TargetCreativeIDs,
		"switch_map":          switchResult.SwitchMap,
		"next_step":           "switch_completed",
	})
}

func (h *Handler) ExecuteContingency(w http.ResponseWriter, r *http.Request) {
	var req executeContingencyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		writeErr(w, http.StatusBadRequest, "invalid_json")
		return
	}

	req.IncidentUUID = strings.TrimSpace(req.IncidentUUID)
	if req.IncidentUUID == "" {
		writeErr(w, http.StatusBadRequest, "missing_contingency_incident_uuid")
		return
	}

	incident, ok := h.loadContingencyIncident(w, r, req.IncidentUUID)
	if !ok {
		return
	}
	if !h.requireAdAccountWriteAccess(w, r, incident.SourceAdAccountID) {
		return
	}

	h.executeContingencyIncident(w, r, incident, parseContingencyMaxAttempts(req.MaxAttempts))
}

func (h *Handler) RetryContingencyIncident(w http.ResponseWriter, r *http.Request) {
	incidentUUID := strings.TrimSpace(chi.URLParam(r, "incident_uuid"))
	if incidentUUID == "" {
		writeErr(w, http.StatusBadRequest, "missing_contingency_incident_uuid")
		return
	}

	var req struct {
		MaxAttempts int `json:"max_attempts"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		writeErr(w, http.StatusBadRequest, "invalid_json")
		return
	}

	incident, ok := h.loadContingencyIncident(w, r, incidentUUID)
	if !ok {
		return
	}
	if !h.requireAdAccountAccessWithRoles(w, r, incident.SourceAdAccountID, contingencyManualRoles) {
		return
	}

	h.executeContingencyIncident(w, r, incident, parseContingencyMaxAttempts(req.MaxAttempts))
}

func (h *Handler) GetContingencyIncident(w http.ResponseWriter, r *http.Request) {
	incidentUUID := strings.TrimSpace(chi.URLParam(r, "incident_uuid"))
	if incidentUUID == "" {
		writeErr(w, http.StatusBadRequest, "missing_contingency_incident_uuid")
		return
	}

	incident, ok := h.loadContingencyIncident(w, r, incidentUUID)
	if !ok {
		return
	}
	if !h.requireAdAccountAccess(w, r, incident.SourceAdAccountID) {
		return
	}

	limit := 20
	if raw := strings.TrimSpace(r.URL.Query().Get("execution_limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "invalid_limit")
			return
		}
		limit = parseContingencyLimit(parsed)
	}

	executions, err := h.Store.ListContingencyExecutionsByIncident(r.Context(), incidentUUID, limit)
	if err != nil {
		writeErrCause(w, http.StatusInternalServerError, "failed_to_list_contingency_executions", err)
		return
	}

	switchMaps, err := h.Store.ListEntitySwitchMapByIncident(r.Context(), incidentUUID, limit)
	if err != nil {
		writeErrCause(w, http.StatusInternalServerError, "failed_to_list_contingency_switch_maps", err)
		return
	}

	var latestExecution any = nil
	if len(executions) > 0 {
		latestExecution = executions[0]
	}

	var latestSwitchMap any = nil
	if len(switchMaps) > 0 {
		latestSwitchMap = switchMaps[0]
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"incident":          incident,
		"latest_execution":  latestExecution,
		"executions":        executions,
		"execution_count":   len(executions),
		"latest_switch_map": latestSwitchMap,
		"switch_maps":       switchMaps,
		"switch_map_count":  len(switchMaps),
	})
}

func (h *Handler) CloseContingencyIncident(w http.ResponseWriter, r *http.Request) {
	incidentUUID := strings.TrimSpace(chi.URLParam(r, "incident_uuid"))
	if incidentUUID == "" {
		writeErr(w, http.StatusBadRequest, "missing_contingency_incident_uuid")
		return
	}

	var req closeContingencyIncidentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		writeErr(w, http.StatusBadRequest, "invalid_json")
		return
	}

	finalStatus, ok := parseContingencyCloseStatus(req.Status)
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid_contingency_close_status")
		return
	}

	incident, ok := h.loadContingencyIncident(w, r, incidentUUID)
	if !ok {
		return
	}
	if !h.requireAdAccountAccessWithRoles(w, r, incident.SourceAdAccountID, contingencyManualRoles) {
		return
	}

	updated, err := h.Store.CloseContingencyIncident(r.Context(), storage.CloseContingencyIncidentInput{
		IncidentUUID: incidentUUID,
		FinalStatus:  finalStatus,
		ReasonDetail: req.ReasonDetail,
	})
	if err != nil {
		msg := strings.ToLower(strings.TrimSpace(err.Error()))
		switch {
		case strings.Contains(msg, "already closed"):
			writeErr(w, http.StatusConflict, "contingency_incident_already_closed")
			return
		case strings.Contains(msg, "in progress"):
			writeErr(w, http.StatusConflict, "contingency_incident_close_conflict")
			return
		default:
			writeErrCause(w, http.StatusInternalServerError, "failed_to_close_contingency_incident", err)
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"incident_uuid":      updated.IncidentUUID,
		"incident_status":    updated.Status,
		"source_ad_account":  updated.SourceAdAccountID,
		"source_campaign_id": updated.SourceCampaignID,
		"closed_at":          updated.ClosedAt,
		"reason_detail":      updated.ReasonDetail,
	})
}

func (h *Handler) ListContingencyExecutions(w http.ResponseWriter, r *http.Request) {
	incidentUUID := strings.TrimSpace(r.URL.Query().Get("incident_uuid"))
	if incidentUUID == "" {
		writeErr(w, http.StatusBadRequest, "missing_contingency_incident_uuid")
		return
	}

	incident, err := h.Store.GetContingencyIncidentByUUID(r.Context(), incidentUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "contingency_incident_not_found")
			return
		}
		writeErrCause(w, http.StatusInternalServerError, "failed_to_get_contingency_incident", err)
		return
	}

	if !h.requireAdAccountAccess(w, r, incident.SourceAdAccountID) {
		return
	}

	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "invalid_limit")
			return
		}
		limit = parseContingencyLimit(parsed)
	}

	items, err := h.Store.ListContingencyExecutionsByIncident(r.Context(), incidentUUID, limit)
	if err != nil {
		writeErrCause(w, http.StatusInternalServerError, "failed_to_list_contingency_executions", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"incident_uuid": incidentUUID,
		"count":         len(items),
		"executions":    items,
	})
}
