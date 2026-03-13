package httpapi

import (
	"encoding/json"
	"strings"
	"time"

	"creative-service/internal/storage"
)

type statusRecordView struct {
	EntityType    string          `json:"entity_type"`
	EntityID      string          `json:"entity_id"`
	AdAccountID   string          `json:"ad_account_id"`
	Status        string          `json:"status"`
	RawPayload    json.RawMessage `json:"raw_payload"`
	SyncedAt      time.Time       `json:"synced_at"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
	Source        string          `json:"source,omitempty"`
	WebhookField  string          `json:"webhook_field,omitempty"`
	WebhookLevel  string          `json:"webhook_level,omitempty"`
	ErrorCode     string          `json:"error_code,omitempty"`
	ErrorSummary  string          `json:"error_summary,omitempty"`
	ErrorMessage  string          `json:"error_message,omitempty"`
	StatusReason  string          `json:"status_reason,omitempty"`
	GraphStatus   string          `json:"graph_status,omitempty"`
	GraphName     string          `json:"graph_name,omitempty"`
	SourceAdID    string          `json:"source_ad_id,omitempty"`
	SourceAdState string          `json:"source_ad_status,omitempty"`
}

type statusRawInfo struct {
	Source        string
	WebhookField  string
	WebhookLevel  string
	ErrorCode     string
	ErrorSummary  string
	ErrorMessage  string
	StatusReason  string
	GraphStatus   string
	GraphName     string
	SourceAdID    string
	SourceAdState string
}

func buildStatusViews(rows []storage.EntityStatusRecord) []statusRecordView {
	out := make([]statusRecordView, 0, len(rows))
	for _, row := range rows {
		info := extractStatusRawInfo(row.RawPayload)

		out = append(out, statusRecordView{
			EntityType:    row.EntityType,
			EntityID:      row.EntityID,
			AdAccountID:   row.AdAccountID,
			Status:        row.Status,
			RawPayload:    row.RawPayload,
			SyncedAt:      row.SyncedAt,
			CreatedAt:     row.CreatedAt,
			UpdatedAt:     row.UpdatedAt,
			Source:        info.Source,
			WebhookField:  info.WebhookField,
			WebhookLevel:  info.WebhookLevel,
			ErrorCode:     info.ErrorCode,
			ErrorSummary:  info.ErrorSummary,
			ErrorMessage:  info.ErrorMessage,
			StatusReason:  info.StatusReason,
			GraphStatus:   info.GraphStatus,
			GraphName:     info.GraphName,
			SourceAdID:    info.SourceAdID,
			SourceAdState: info.SourceAdState,
		})
	}
	return out
}

func extractStatusRawInfo(raw json.RawMessage) statusRawInfo {
	info := statusRawInfo{}
	if len(raw) == 0 {
		return info
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return info
	}

	info.Source = webhookAnyToString(payload["source"])

	if graph, ok := payload["graph"].(map[string]any); ok {
		info.GraphStatus = firstNonEmptyStatus(
			webhookAnyToString(graph["effective_status"]),
			webhookAnyToString(graph["configured_status"]),
			webhookAnyToString(graph["status"]),
		)
		info.GraphName = webhookAnyToString(graph["name"])
	}
	if info.GraphStatus == "" {
		info.GraphStatus = firstNonEmptyStatus(
			webhookAnyToString(payload["effective_status"]),
			webhookAnyToString(payload["configured_status"]),
			webhookAnyToString(payload["status"]),
		)
	}
	if info.GraphName == "" {
		info.GraphName = webhookAnyToString(payload["name"])
	}

	if webhook, ok := payload["webhook"].(map[string]any); ok {
		info.WebhookField = webhookAnyToString(webhook["field"])
		if value, ok := webhook["value"].(map[string]any); ok {
			info.WebhookLevel = strings.ToUpper(webhookAnyToString(value["level"]))
			info.ErrorCode = webhookAnyToString(value["error_code"])
			info.ErrorSummary = webhookAnyToString(value["error_summary"])
			info.ErrorMessage = webhookAnyToString(value["error_message"])
		}
	}
	if info.ErrorCode == "" {
		info.ErrorCode = webhookAnyToString(payload["error_code"])
	}
	if info.ErrorSummary == "" {
		info.ErrorSummary = webhookAnyToString(payload["error_summary"])
	}
	if info.ErrorMessage == "" {
		info.ErrorMessage = webhookAnyToString(payload["error_message"])
	}

	info.SourceAdID = webhookAnyToString(payload["source_ad_id"])
	info.SourceAdState = webhookAnyToString(payload["source_ad_status"])

	info.StatusReason = firstNonEmptyStatusReason(
		webhookAnyToString(payload["status_reason"]),
		info.ErrorMessage,
		info.ErrorSummary,
		extractReasonFromMap(payload),
	)
	return info
}

func firstNonEmptyStatusReason(values ...string) string {
	for _, value := range values {
		v := strings.TrimSpace(value)
		if v != "" {
			return v
		}
	}
	return ""
}

func extractReasonFromMap(payload map[string]any) string {
	keys := []string{
		"ad_review_feedback",
		"issues_info",
		"review_feedback",
		"rejection_reasons",
		"disapproval_reasons",
		"recommendations",
		"message",
		"description",
	}

	for _, key := range keys {
		if value, ok := payload[key]; ok {
			if reason := extractReasonFromAny(value); reason != "" {
				return reason
			}
		}
	}

	return ""
}

func extractReasonFromAny(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case map[string]any:
		// Prioriza chaves mais comuns de motivo.
		for _, key := range []string{"error_message", "error_summary", "message", "summary", "description", "reason", "global"} {
			if v, ok := typed[key]; ok {
				if reason := extractReasonFromAny(v); reason != "" {
					return reason
				}
			}
		}
		// Fallback: varre todo o mapa.
		for _, v := range typed {
			if reason := extractReasonFromAny(v); reason != "" {
				return reason
			}
		}
	case []any:
		for _, v := range typed {
			if reason := extractReasonFromAny(v); reason != "" {
				return reason
			}
		}
	}
	return ""
}
