package httpapi

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

type adAccountMetaStatusResponse struct {
	AdAccountID string                      `json:"ad_account_id"`
	System      adAccountSystemStatusView   `json:"system"`
	Meta        adAccountMetaStatusEnvelope `json:"meta"`
}

type adAccountSystemStatusView struct {
	AccountName string  `json:"account_name"`
	AccountOn   bool    `json:"account_on"`
	BMUUID      *string `json:"bm_uuid,omitempty"`
	BMID        string  `json:"bm_id,omitempty"`
	BMOn        bool    `json:"bm_on"`
	BMKnown     bool    `json:"bm_known"`
}

type adAccountMetaStatusEnvelope struct {
	FetchedAt time.Time                  `json:"fetched_at"`
	Account   adAccountMetaRuntimeStatus `json:"account"`
	BM        bmMetaRuntimeStatus        `json:"bm"`
}

type adAccountMetaRuntimeStatus struct {
	Available         bool   `json:"available"`
	ID                string `json:"id,omitempty"`
	Name              string `json:"name,omitempty"`
	StatusCode        *int   `json:"status_code,omitempty"`
	Status            string `json:"status,omitempty"`
	DisableReasonCode *int   `json:"disable_reason_code,omitempty"`
	DisableReason     string `json:"disable_reason,omitempty"`
	IsActive          *bool  `json:"is_active,omitempty"`
	ErrorCode         string `json:"error_code,omitempty"`
}

type bmMetaRuntimeStatus struct {
	Available          bool   `json:"available"`
	ID                 string `json:"id,omitempty"`
	Name               string `json:"name,omitempty"`
	VerificationStatus string `json:"verification_status,omitempty"`
	Verification       string `json:"verification,omitempty"`
	IsVerified         *bool  `json:"is_verified,omitempty"`
	ErrorCode          string `json:"error_code,omitempty"`
}

func (h *Handler) GetAdAccountMetaStatus(w http.ResponseWriter, r *http.Request) {
	adAccountID := strings.TrimSpace(chi.URLParam(r, "ad_account_id"))
	if adAccountID == "" {
		writeErr(w, http.StatusBadRequest, "missing_ad_account_id")
		return
	}

	if !h.requireAdAccountAccess(w, r, adAccountID) {
		return
	}

	adAccount, err := h.Store.GetAdAccount(r.Context(), adAccountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "not_found")
			return
		}
		writeErrCause(w, http.StatusInternalServerError, "failed_to_fetch_meta_account_status", err)
		return
	}

	resp := adAccountMetaStatusResponse{
		AdAccountID: adAccount.AdAccountID,
		System: adAccountSystemStatusView{
			AccountName: adAccount.AdAccountName,
			AccountOn:   adAccount.IsActive,
			BMUUID:      adAccount.BMUUID,
			BMID:        strings.TrimSpace(adAccount.BMID),
			BMOn:        adAccount.BMIsActive,
			BMKnown:     adAccount.BMUUID != nil && strings.TrimSpace(*adAccount.BMUUID) != "",
		},
		Meta: adAccountMetaStatusEnvelope{
			FetchedAt: time.Now().UTC(),
			Account: adAccountMetaRuntimeStatus{
				Available: false,
				Status:    "Indisponível",
			},
			BM: bmMetaRuntimeStatus{
				Available:    false,
				Verification: "Indisponível",
			},
		},
	}

	mc, err := h.metaClientForAdAccount(r.Context(), adAccount.AdAccountID)
	if err != nil {
		resp.Meta.Account.ErrorCode = "meta_client_unavailable"
		resp.Meta.BM.ErrorCode = "meta_client_unavailable"
		writeJSON(w, http.StatusOK, resp)
		return
	}

	accountObj, err := mc.GetObject(r.Context(), adAccount.AdAccountID, []string{
		"id",
		"name",
		"account_status",
		"disable_reason",
	})
	if err != nil {
		log.Printf("meta_account_status_fetch_failed ad_account=%s err=%v", adAccount.AdAccountID, err)
		resp.Meta.Account.ErrorCode = "meta_account_request_failed"
	} else {
		resp.Meta.Account = buildMetaAccountStatusView(accountObj)
	}

	bmID := strings.TrimSpace(adAccount.BMID)
	if bmID == "" {
		resp.Meta.BM.Verification = "Não vinculada"
		resp.Meta.BM.ErrorCode = "bm_not_linked"
		writeJSON(w, http.StatusOK, resp)
		return
	}

	bmObj, err := mc.GetObject(r.Context(), bmID, []string{
		"id",
		"name",
		"verification_status",
	})
	if err != nil {
		log.Printf("meta_bm_status_fetch_failed bm_id=%s ad_account=%s err=%v", bmID, adAccount.AdAccountID, err)
		resp.Meta.BM.ID = bmID
		resp.Meta.BM.ErrorCode = "meta_bm_request_failed"
		writeJSON(w, http.StatusOK, resp)
		return
	}

	resp.Meta.BM = buildMetaBMStatusView(bmObj)
	if resp.Meta.BM.ID == "" {
		resp.Meta.BM.ID = bmID
	}

	writeJSON(w, http.StatusOK, resp)
}

func buildMetaAccountStatusView(obj map[string]any) adAccountMetaRuntimeStatus {
	out := adAccountMetaRuntimeStatus{
		Available: true,
		ID:        strings.TrimSpace(webhookAnyToString(obj["id"])),
		Name:      strings.TrimSpace(webhookAnyToString(obj["name"])),
	}

	if statusCode, ok := parseAnyInt(obj["account_status"]); ok {
		out.StatusCode = intPtr(statusCode)
		out.Status = accountStatusLabel(statusCode)
		isActive := statusCode == 1
		out.IsActive = boolPtr(isActive)
	} else {
		out.Status = "Não informado"
	}

	if disableCode, ok := parseAnyInt(obj["disable_reason"]); ok {
		out.DisableReasonCode = intPtr(disableCode)
		out.DisableReason = accountDisableReasonLabel(disableCode)
	} else {
		out.DisableReason = "Não informado"
	}

	return out
}

func buildMetaBMStatusView(obj map[string]any) bmMetaRuntimeStatus {
	raw := strings.ToUpper(strings.TrimSpace(webhookAnyToString(obj["verification_status"])))
	out := bmMetaRuntimeStatus{
		Available:          true,
		ID:                 strings.TrimSpace(webhookAnyToString(obj["id"])),
		Name:               strings.TrimSpace(webhookAnyToString(obj["name"])),
		VerificationStatus: raw,
		Verification:       bmVerificationLabel(raw),
	}

	switch raw {
	case "VERIFIED", "BUSINESS_VERIFIED":
		out.IsVerified = boolPtr(true)
	case "UNVERIFIED", "NOT_VERIFIED", "PENDING", "IN_REVIEW", "REJECTED":
		out.IsVerified = boolPtr(false)
	}

	if out.Verification == "" {
		out.Verification = "Não informado"
	}
	return out
}

func parseAnyInt(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int8:
		return int(typed), true
	case int16:
		return int(typed), true
	case int32:
		return int(typed), true
	case int64:
		return int(typed), true
	case uint:
		return int(typed), true
	case uint8:
		return int(typed), true
	case uint16:
		return int(typed), true
	case uint32:
		return int(typed), true
	case uint64:
		return int(typed), true
	case float32:
		return int(typed), true
	case float64:
		return int(typed), true
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil {
			return 0, false
		}
		return n, true
	default:
		s := webhookAnyToString(value)
		if s == "" {
			return 0, false
		}
		n, err := strconv.Atoi(s)
		if err != nil {
			return 0, false
		}
		return n, true
	}
}

func accountStatusLabel(code int) string {
	switch code {
	case 1:
		return "Ativa"
	case 2:
		return "Desativada"
	case 3:
		return "Pendente de cobrança"
	case 7:
		return "Em revisão de risco"
	case 8:
		return "Pendente de liquidação"
	case 9:
		return "Período de carência"
	case 100:
		return "Pendente de encerramento"
	case 101:
		return "Encerrada"
	default:
		return "Indefinida (" + strconv.Itoa(code) + ")"
	}
}

func accountDisableReasonLabel(code int) string {
	switch code {
	case 0:
		return "Sem restrição"
	case 1:
		return "Titular da conta desativado"
	case 2:
		return "Risco detectado"
	case 3:
		return "Fim da conta"
	case 4:
		return "Limite de gasto atingido"
	case 5:
		return "Atraso de pagamento"
	case 6:
		return "Violação de política"
	case 7:
		return "Desativada por condição desconhecida"
	case 8:
		return "Ação da equipe de fraude"
	case 9:
		return "Contingência indisponível"
	case 10:
		return "Ação de compliance"
	default:
		return "Não informado (" + strconv.Itoa(code) + ")"
	}
}

func bmVerificationLabel(raw string) string {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "VERIFIED", "BUSINESS_VERIFIED":
		return "Verificada"
	case "UNVERIFIED", "NOT_VERIFIED":
		return "Não verificada"
	case "PENDING":
		return "Pendente"
	case "IN_REVIEW":
		return "Em revisão"
	case "REJECTED":
		return "Reprovada"
	default:
		return ""
	}
}

func boolPtr(v bool) *bool {
	return &v
}

func intPtr(v int) *int {
	return &v
}
