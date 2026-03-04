package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"creative-service/internal/meta"
	"creative-service/internal/storage"
)

type metaWebhookPayload struct {
	Object string             `json:"object"`
	Entry  []metaWebhookEntry `json:"entry"`
}

type metaWebhookEntry struct {
	ID      string              `json:"id"`
	Time    int64               `json:"time"`
	Changes []metaWebhookChange `json:"changes"`
}

type metaWebhookChange struct {
	Field string         `json:"field"`
	Value map[string]any `json:"value"`
}

type metaWebhookSyncTarget struct {
	AdAccountID string
	EntityType  string
	EntityID    string
	Field       string
	Value       map[string]any
}

type metaWebhookProcessResult struct {
	ReceivedChanges int      `json:"received_changes"`
	IgnoredChanges  int      `json:"ignored_changes"`
	UniqueTargets   int      `json:"unique_targets"`
	SyncedTargets   int      `json:"synced_targets"`
	FailedTargets   int      `json:"failed_targets"`
	SampleErrors    []string `json:"sample_errors,omitempty"`
}

func (h *Handler) MetaWebhookVerify(w http.ResponseWriter, r *http.Request) {
	if strings.TrimSpace(h.MetaWebhookVerifyToken) == "" {
		writeErr(w, http.StatusServiceUnavailable, "meta_webhook_not_configured")
		return
	}

	mode := strings.TrimSpace(r.URL.Query().Get("hub.mode"))
	verifyToken := strings.TrimSpace(r.URL.Query().Get("hub.verify_token"))
	challenge := strings.TrimSpace(r.URL.Query().Get("hub.challenge"))
	if mode != "subscribe" || verifyToken == "" || challenge == "" {
		writeErr(w, http.StatusBadRequest, "invalid_webhook_payload")
		return
	}

	if verifyToken != h.MetaWebhookVerifyToken {
		writeErr(w, http.StatusForbidden, "invalid_webhook_verify_token")
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(challenge))
}

func (h *Handler) MetaWebhookAdAccount(w http.ResponseWriter, r *http.Request) {
	if strings.TrimSpace(h.MetaWebhookAppSecret) == "" {
		writeErr(w, http.StatusServiceUnavailable, "meta_webhook_not_configured")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_webhook_payload")
		return
	}
	if len(body) == 0 {
		writeErr(w, http.StatusBadRequest, "invalid_webhook_payload")
		return
	}

	signature := strings.TrimSpace(r.Header.Get("X-Hub-Signature-256"))
	if !metaWebhookSignatureValid(h.MetaWebhookAppSecret, body, signature) {
		writeErr(w, http.StatusUnauthorized, "invalid_webhook_signature")
		return
	}

	payloads, err := parseMetaWebhookPayloads(body)
	if err != nil {
		writeErrCause(w, http.StatusBadRequest, "invalid_webhook_payload", err)
		return
	}

	result, upserts := h.buildWebhookStatusUpserts(r.Context(), payloads)
	if len(upserts) > 0 {
		if err := h.Store.UpsertEntityStatuses(r.Context(), upserts); err != nil {
			writeErrCause(w, http.StatusInternalServerError, "failed_to_sync_webhook_status", err)
			return
		}
		result.SyncedTargets = len(upserts)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"result": result,
	})
}

func (h *Handler) buildWebhookStatusUpserts(
	ctx context.Context,
	payloads []metaWebhookPayload,
) (metaWebhookProcessResult, []storage.EntityStatusUpsert) {
	result := metaWebhookProcessResult{}
	targetsByKey := map[string]metaWebhookSyncTarget{}

	for _, payload := range payloads {
		if strings.ToLower(strings.TrimSpace(payload.Object)) != "ad_account" {
			continue
		}

		for _, entry := range payload.Entry {
			for _, change := range entry.Changes {
				result.ReceivedChanges++
				target, ok := parseWebhookTarget(entry.ID, change)
				if !ok {
					result.IgnoredChanges++
					continue
				}

				key := target.AdAccountID + "|" + target.EntityType + "|" + target.EntityID
				targetsByKey[key] = target
			}
		}
	}

	result.UniqueTargets = len(targetsByKey)
	if len(targetsByKey) == 0 {
		return result, nil
	}

	clientsByAdAccount := map[string]*meta.Client{}
	upserts := make([]storage.EntityStatusUpsert, 0, len(targetsByKey))

	for _, target := range targetsByKey {
		mc, ok := clientsByAdAccount[target.AdAccountID]
		if !ok {
			client, err := h.metaClientForAdAccount(ctx, target.AdAccountID)
			if err != nil {
				result.FailedTargets++
				result.SampleErrors = appendWebhookSampleError(
					result.SampleErrors,
					fmt.Sprintf("conta %s: %v", target.AdAccountID, err),
				)
				continue
			}
			mc = client
			clientsByAdAccount[target.AdAccountID] = mc
		}

		graphObject, graphStatus, err := fetchGraphEntityStatus(ctx, mc, target.EntityType, target.EntityID)
		if err != nil {
			result.FailedTargets++
			result.SampleErrors = appendWebhookSampleError(
				result.SampleErrors,
				fmt.Sprintf("%s %s: %v", target.EntityType, target.EntityID, err),
			)
			continue
		}

		status := firstNonEmptyStatus(
			graphStatus,
			normalizeWebhookStatus(webhookAnyToString(target.Value["status_name"])),
			statusFromWebhookField(target.Field),
		)
		if status == "" {
			status = "UNKNOWN"
		}

		rawPayload, err := json.Marshal(map[string]any{
			"source": "meta_webhook",
			"webhook": map[string]any{
				"field": target.Field,
				"value": target.Value,
			},
			"graph":           graphObject,
			"resolved_status": status,
		})
		if err != nil {
			result.FailedTargets++
			result.SampleErrors = appendWebhookSampleError(
				result.SampleErrors,
				fmt.Sprintf("%s %s: %v", target.EntityType, target.EntityID, err),
			)
			continue
		}

		upserts = append(upserts, storage.EntityStatusUpsert{
			EntityType:  target.EntityType,
			EntityID:    target.EntityID,
			AdAccountID: target.AdAccountID,
			Status:      status,
			RawPayload:  rawPayload,
		})
	}

	return result, upserts
}

func parseMetaWebhookPayloads(body []byte) ([]metaWebhookPayload, error) {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return nil, fmt.Errorf("empty body")
	}

	if strings.HasPrefix(trimmed, "[") {
		var payloads []metaWebhookPayload
		if err := json.Unmarshal(body, &payloads); err != nil {
			return nil, err
		}
		return payloads, nil
	}

	var payload metaWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	return []metaWebhookPayload{payload}, nil
}

func parseWebhookTarget(entryID string, change metaWebhookChange) (metaWebhookSyncTarget, bool) {
	adAccountID := normalizeWebhookAdAccountID(entryID)
	if adAccountID == "" {
		adAccountID = normalizeWebhookAdAccountID(webhookAnyToString(change.Value["ad_account_id"]))
	}

	entityID := strings.TrimSpace(webhookAnyToString(change.Value["id"]))
	if entityID == "" || adAccountID == "" {
		return metaWebhookSyncTarget{}, false
	}

	entityType, ok := entityTypeFromWebhookLevel(webhookAnyToString(change.Value["level"]))
	if !ok {
		return metaWebhookSyncTarget{}, false
	}

	return metaWebhookSyncTarget{
		AdAccountID: adAccountID,
		EntityType:  entityType,
		EntityID:    entityID,
		Field:       strings.TrimSpace(change.Field),
		Value:       change.Value,
	}, true
}

func (h *Handler) metaClientForAdAccount(ctx context.Context, adAccountID string) (*meta.Client, error) {
	if h.Store == nil || h.BM == nil || h.Ads == nil {
		return nil, fmt.Errorf("webhook dependencies unavailable")
	}

	adAccount, err := h.Store.GetAdAccount(ctx, adAccountID)
	if err != nil {
		return nil, err
	}

	bmCfg, err := h.BM.GetBMConfigByAdAccountID(ctx, adAccount.AdAccountID)
	if err != nil {
		return nil, err
	}

	token, err := h.Ads.Tokens.Resolve(bmCfg.TokenRef)
	if err != nil {
		return nil, err
	}

	return meta.New(h.Ads.BaseURL, h.Ads.APIVersion, token, h.Ads.HTTPTimeout), nil
}

func fetchGraphEntityStatus(
	ctx context.Context,
	mc *meta.Client,
	entityType, entityID string,
) (map[string]any, string, error) {
	graphObject, err := mc.GetObject(ctx, entityID, graphFieldsForEntityType(entityType))
	if err != nil {
		return nil, "", err
	}

	status := firstNonEmptyStatus(
		normalizeWebhookStatus(webhookAnyToString(graphObject["effective_status"])),
		normalizeWebhookStatus(webhookAnyToString(graphObject["configured_status"])),
		normalizeWebhookStatus(webhookAnyToString(graphObject["status"])),
	)

	return graphObject, status, nil
}

func graphFieldsForEntityType(entityType string) []string {
	switch entityType {
	case "campaign", "adset", "ad":
		return []string{"id", "name", "status", "configured_status", "effective_status"}
	case "creative":
		return []string{"id", "name", "status"}
	default:
		return []string{"id", "status"}
	}
}

func entityTypeFromWebhookLevel(rawLevel string) (string, bool) {
	switch strings.ToUpper(strings.TrimSpace(rawLevel)) {
	case "CAMPAIGN":
		return "campaign", true
	case "ADSET", "AD_SET", "ADGROUP":
		return "adset", true
	case "AD":
		return "ad", true
	case "CREATIVE", "ADCREATIVE", "AD_CREATIVE":
		return "creative", true
	default:
		return "", false
	}
}

func normalizeWebhookAdAccountID(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(raw), "act_") {
		return "act_" + strings.TrimSpace(raw[4:])
	}
	return "act_" + raw
}

func normalizeWebhookStatus(raw string) string {
	return strings.ToUpper(strings.TrimSpace(raw))
}

func firstNonEmptyStatus(values ...string) string {
	for _, value := range values {
		if normalized := normalizeWebhookStatus(value); normalized != "" {
			return normalized
		}
	}
	return ""
}

func statusFromWebhookField(field string) string {
	switch strings.TrimSpace(field) {
	case "with_issues_ad_objects":
		return "WITH_ISSUES"
	case "in_process_ad_objects":
		return "IN_PROCESS"
	default:
		return ""
	}
}

func webhookAnyToString(v any) string {
	switch value := v.(type) {
	case string:
		return strings.TrimSpace(value)
	case json.Number:
		return strings.TrimSpace(value.String())
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(value), 'f', -1, 32)
	case int:
		return strconv.Itoa(value)
	case int64:
		return strconv.FormatInt(value, 10)
	case uint64:
		return strconv.FormatUint(value, 10)
	default:
		return ""
	}
}

func metaWebhookSignatureValid(appSecret string, body []byte, signatureHeader string) bool {
	if strings.TrimSpace(appSecret) == "" {
		return false
	}

	parts := strings.SplitN(strings.TrimSpace(signatureHeader), "=", 2)
	if len(parts) != 2 || strings.ToLower(strings.TrimSpace(parts[0])) != "sha256" {
		return false
	}

	sig, err := hex.DecodeString(strings.TrimSpace(parts[1]))
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, []byte(appSecret))
	_, _ = mac.Write(body)
	expected := mac.Sum(nil)

	return hmac.Equal(sig, expected)
}

func appendWebhookSampleError(errors []string, value string) []string {
	if len(errors) >= 5 {
		return errors
	}
	return append(errors, value)
}
