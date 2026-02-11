package httpapi

import (
	"encoding/json"
	"io"
	"net/http"

	"creative-service/internal/service"
	"creative-service/internal/storage"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	CreativeSync *service.CreativeSyncService
	Store        *storage.Store
	Campaigns    *service.CampaignService
	AdSets       *service.AdSetService
	Ads          *service.AdService
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (h *Handler) ListClients(w http.ResponseWriter, r *http.Request) {
	clients, err := h.Store.ListClients(r.Context())
	if err != nil {
		writeErr(w, 500, "failed to list clients")
		return
	}

	writeJSON(w, 200, map[string]any{
		"clients": clients,
		"count":   len(clients),
	})
}

func (h *Handler) CreateImageCreative(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeErr(w, 400, "invalid_multipart"); return
	}
	
	// Aceitar ad_account_id (novo) ou client_id (deprecated)
	adAccountID := r.FormValue("ad_account_id")
	if adAccountID == "" {
		adAccountID = r.FormValue("client_id") // Fallback para compatibilidade
	}
	if adAccountID == "" { 
		writeErr(w, 400, "missing_ad_account_id"); 
		return 
	}

	file, hdr, err := r.FormFile("image")
	if err != nil { writeErr(w, 400, "missing_image"); return }
	defer file.Close()
	b, _ := io.ReadAll(file)

	out, err := h.CreativeSync.CreateImageCreative(r.Context(), service.ImageCreativeInput{
		AdAccountID:  adAccountID,
		ClientID:     adAccountID, // Manter por compatibilidade
		Name:         r.FormValue("name"),
		Link:         r.FormValue("link"),
		Message:      r.FormValue("message"),
		Headline:     r.FormValue("headline"),
		Description:  r.FormValue("description"),
		ImageName:    hdr.Filename,
		ImageBytes:   b,
	})
	if err != nil { writeErr(w, 400, err.Error()); return }
	writeJSON(w, 200, out)
}

func (h *Handler) CreateVideoCreative(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(1024 << 20); err != nil {
		writeErr(w, 400, "invalid_multipart"); return
	}

	// Aceitar ad_account_id (novo) ou client_id (deprecated)
	adAccountID := r.FormValue("ad_account_id")
	if adAccountID == "" {
		adAccountID = r.FormValue("client_id") // Fallback para compatibilidade
	}
	if adAccountID == "" {
		writeErr(w, 400, "missing_ad_account_id"); 
		return
	}
	
	videoFile, videoHeader, err := r.FormFile("video")
	if err != nil { 
		writeErr(w, 400, "missing_video"); 
		return 
	}
	defer videoFile.Close()
	videoBytes, _ := io.ReadAll(videoFile)

	thumbFile, thumbHeader, err := r.FormFile("thumbnail")
	if err != nil {
		writeErr(w, 400, "missing_thumbnail")
		return
	}
	defer thumbFile.Close()
	thumbBytes, _ := io.ReadAll(thumbFile)

	out, err := h.CreativeSync.CreateVideoCreative(r.Context(), service.VideoCreativeInput{
		AdAccountID:  adAccountID,
		ClientID:     adAccountID, // Manter por compatibilidade
		Name:         r.FormValue("name"),
		Link:         r.FormValue("link"),
		Message:      r.FormValue("message"),
		Headline:     r.FormValue("headline"),
		Description:  r.FormValue("description"),
		VideoName:    videoHeader.Filename,
		VideoBytes:   videoBytes,
		ThumbName:    thumbHeader.Filename,
		ThumbBytes:   thumbBytes,
	})
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}

	writeJSON(w, 200, out)
}

func (h *Handler) CreateCampaign(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ClientID            string   `json:"client_id"`             // Deprecated
		AdAccountID         string   `json:"ad_account_id"`         // Usar este
		Name                string   `json:"name"`
		Objective           string   `json:"objective"`
		Status              string   `json:"status"`
		SpecialAdCategories []string `json:"special_ad_categories"`
		BuyingType          string   `json:"buying_type"`
		IsAdSetBudgetSharingEnabled bool `json:"is_adset_budget_sharing_enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, "invalid_json"); return
	}
	
	// Aceitar ad_account_id ou client_id (compatibilidade)
	adAccountID := req.AdAccountID
	if adAccountID == "" {
		adAccountID = req.ClientID
	}
	if adAccountID == "" { writeErr(w, 400, "missing_ad_account_id"); return }
	if req.Name == "" { writeErr(w, 400, "missing_name"); return }
	if req.Objective == "" { writeErr(w, 400, "missing_objective"); return }
	if req.Status == "" { req.Status = "PAUSED" }
	if req.SpecialAdCategories == nil { req.SpecialAdCategories = []string{} }
	if req.BuyingType == "" { req.BuyingType = "AUCTION" }

	out, err := h.Campaigns.CreateCampaign(r.Context(), service.CreateCampaignInput{
		AdAccountID:         adAccountID,
		Name:                req.Name,
		Objective:           req.Objective,
		Status:              req.Status,
		SpecialAdCategories: req.SpecialAdCategories,
		BuyingType:			 req.BuyingType,
		IsAdSetBudgetSharingEnabled: req.IsAdSetBudgetSharingEnabled,
	})
	if err != nil { writeErr(w, 400, err.Error()); return }
	writeJSON(w, 200, out)
}

func (h *Handler) CreateAdSet(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ClientID         string         `json:"client_id"`          // Deprecated
		AdAccountID      string         `json:"ad_account_id"`      // Usar este
		CampaignID       string         `json:"campaign_id"`
		Name             string         `json:"name"`
		BillingEvent     string         `json:"billing_event"`
		OptimizationGoal string         `json:"optimization_goal"`
		BidAmount        int            `json:"bid_amount"`
		DailyBudget      int            `json:"daily_budget"`
		Targeting        map[string]any `json:"targeting"`
		Status           string         `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, "invalid_json"); return
	}
	
	// Aceitar ad_account_id ou client_id (compatibilidade)
	adAccountID := req.AdAccountID
	if adAccountID == "" {
		adAccountID = req.ClientID
	}
	if adAccountID == "" { writeErr(w, 400, "missing_ad_account_id"); return }
	if req.CampaignID == "" { writeErr(w, 400, "missing_campaign_id"); return }
	if req.Name == "" { writeErr(w, 400, "missing_name"); return }
	if req.BillingEvent == "" { writeErr(w, 400, "missing_billing_event"); return }
	if req.OptimizationGoal == "" { writeErr(w, 400, "missing_optimization_goal"); return }
	if req.DailyBudget == 0 { writeErr(w, 400, "missing_daily_budget"); return }
	if req.Status == "" { req.Status = "PAUSED" }

	out, err := h.AdSets.CreateAdSet(r.Context(), service.CreateAdSetInput{
		AdAccountID:      adAccountID,
		CampaignID:       req.CampaignID,
		Name:             req.Name,
		BillingEvent:     req.BillingEvent,
		OptimizationGoal: req.OptimizationGoal,
		BidAmount:        req.BidAmount,
		DailyBudget:      req.DailyBudget,
		Targeting:        req.Targeting,
		Status:           req.Status,
	})
	if err != nil { writeErr(w, 400, err.Error()); return }
	writeJSON(w, 200, out)
}

func (h *Handler) CreateAd(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ClientID   string `json:"client_id"`      // Deprecated
		AdAccountID string `json:"ad_account_id"` // Usar este
		AdSetID    string `json:"adset_id"`
		CreativeID string `json:"creative_id"`
		Name       string `json:"name"`
		Status     string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, "invalid_json"); return
	}
	
	// Aceitar ad_account_id ou client_id (compatibilidade)
	adAccountID := req.AdAccountID
	if adAccountID == "" {
		adAccountID = req.ClientID
	}
	if adAccountID == "" { writeErr(w, 400, "missing_ad_account_id"); return }
	if req.AdSetID == "" { writeErr(w, 400, "missing_adset_id"); return }
	if req.CreativeID == "" { writeErr(w, 400, "missing_creative_id"); return }
	if req.Name == "" { writeErr(w, 400, "missing_name"); return }
	if req.Status == "" { req.Status = "PAUSED" }

	out, err := h.Ads.CreateAd(r.Context(), service.CreateAdInput{
		AdAccountID: adAccountID,
		AdSetID:    req.AdSetID,
		CreativeID: req.CreativeID,
		Name:       req.Name,
		Status:     req.Status,
	})
	if err != nil { writeErr(w, 400, err.Error()); return }
	writeJSON(w, 200, out)
}

func (h *Handler) ListCreatives(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("client_id")
	typeFilter := r.URL.Query().Get("type")

	if typeFilter != "" && typeFilter != "image" && typeFilter != "video" {
		writeErr(w, 400, "invalid_type_filter"); return
	}

	creatives, err := h.Store.ListCreatives(r.Context(), clientID, typeFilter)
	if err != nil {
		writeErr(w, 500, "failed to list creatives"); 
		return 
	}

	writeJSON(w, 200, map[string]any{
		"creatives": creatives,
		"count": len(creatives),
	})
}

func (h *Handler) GetCreative(w http.ResponseWriter, r *http.Request) {
	creativeID := chi.URLParam(r, "creative_id")
	if creativeID == "" { 
		writeErr(w, 400, "missing_creative_id"); 
		return 
	}

	creative, err := h.Store.GetCreative(r.Context(), creativeID)
	if err != nil { 
		writeErr(w, 404, "creative_not_found"); 
		return 
	}

	writeJSON(w, 200, creative)
}

// ListAdAccountsByClient lista todas as ad accounts de um cliente
func (h *Handler) ListAdAccountsByClient(w http.ResponseWriter, r *http.Request) {
	clientUUID := chi.URLParam(r, "client_uuid")
	if clientUUID == "" {
		writeErr(w, 400, "missing_client_uuid")
		return
	}

	adAccounts, err := h.Store.ListAdAccountsByClient(r.Context(), clientUUID)
	if err != nil {
		writeErr(w, 500, "failed to list ad accounts")
		return
	}

	writeJSON(w, 200, map[string]any{
		"ad_accounts": adAccounts,
		"count":       len(adAccounts),
	})
}

// SoftDeleteCreative marca um creative como deletado (soft delete)
func (h *Handler) SoftDeleteCreative(w http.ResponseWriter, r *http.Request) {
	creativeID := chi.URLParam(r, "creative_id")
	if creativeID == "" {
		writeErr(w, 400, "missing_creative_id")
		return
	}

	err := h.Store.SoftDeleteCreative(r.Context(), creativeID)
	if err != nil {
		writeErr(w, 404, err.Error())
		return
	}

	writeJSON(w, 200, map[string]any{
		"message": "creative deleted successfully",
		"creative_id": creativeID,
	})
}

func (h *Handler) ListCampaigns(w http.ResponseWriter, r *http.Request) {
	adAccountID := r.URL.Query().Get("ad_account_id")
	if adAccountID == "" {
		writeErr(w, 400, "missing_ad_account_id")
		return
	}

	out, err := h.Campaigns.ListCampaigns(r.Context(), service.ListCampaignsInput{
		AdAccountID: adAccountID,
	})
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}

	writeJSON(w, 200, out)
}

func (h *Handler) ListAdSets(w http.ResponseWriter, r *http.Request) {
	adAccountID := r.URL.Query().Get("ad_account_id")
	if adAccountID == "" {
		writeErr(w, 400, "missing_ad_account_id")
		return
	}

	out, err := h.AdSets.ListAdSets(r.Context(), service.ListAdSetsInput{
		AdAccountID: adAccountID,
	})
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}

	writeJSON(w, 200, out)
}

func (h *Handler) ListAds(w http.ResponseWriter, r *http.Request) {
	adAccountID := r.URL.Query().Get("ad_account_id")
	if adAccountID == "" {
		writeErr(w, 400, "missing_ad_account_id")
		return
	}

	out, err := h.Ads.ListAds(r.Context(), service.ListAdsInput{
		AdAccountID: adAccountID,
	})
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}

	writeJSON(w, 200, out)
}