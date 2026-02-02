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
	VideoJobs    *service.VideoJobService
	Store        *storage.Store
	Campaigns    *service.CampaignService
	AdSets       *service.AdSetService
	Ads          *service.AdService
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (h *Handler) CreateImageCreative(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeErr(w, 400, "invalid_multipart"); return
	}
	clientID := r.FormValue("client_id")
	if clientID == "" { writeErr(w, 400, "missing_client_id"); return }

	file, hdr, err := r.FormFile("image")
	if err != nil { writeErr(w, 400, "missing_image"); return }
	defer file.Close()
	b, _ := io.ReadAll(file)

	out, err := h.CreativeSync.CreateImageCreative(r.Context(), service.ImageCreativeInput{
		ClientID:     clientID,
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

func (h *Handler) CreateVideoCreativeJob(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(1024 << 20); err != nil {
		writeErr(w, 400, "invalid_multipart"); return
	}
	clientID := r.FormValue("client_id")
	if clientID == "" { writeErr(w, 400, "missing_client_id"); return }

	vf, vh, err := r.FormFile("video")
	if err != nil { writeErr(w, 400, "missing_video"); return }
	defer vf.Close()
	vbytes, _ := io.ReadAll(vf)

	tf, th, err := r.FormFile("thumbnail")
	if err != nil { writeErr(w, 400, "missing_thumbnail"); return }
	defer tf.Close()
	tbytes, _ := io.ReadAll(tf)

	out, err := h.VideoJobs.EnqueueVideoCreativeJob(r.Context(), service.CreateVideoJobInput{
		ClientID:     clientID,
		Name:         r.FormValue("name"),
		Link:         r.FormValue("link"),
		Message:      r.FormValue("message"),
		Headline:     r.FormValue("headline"),
		Description:  r.FormValue("description"),
		CTAType:      r.FormValue("cta_type"),
		VideoName:    vh.Filename,
		VideoBytes:   vbytes,
		ThumbName:    th.Filename,
		ThumbBytes:   tbytes,
	})
	if err != nil { writeErr(w, 400, err.Error()); return }
	writeJSON(w, 202, out)
}

func (h *Handler) GetJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "job_id")
	if jobID == "" { writeErr(w, 400, "missing_job_id"); return }

	j, err := h.Store.GetJob(r.Context(), jobID)
	if err != nil { writeErr(w, 404, "job_not_found"); return }

	resp := map[string]any{
		"job_id":    j.JobID,
		"client_id": j.ClientID,
		"type":      j.JobType,
		"status":    j.Status,
		"result":    json.RawMessage(j.ResultJSON),
		"error":     nil,
	}
	if j.ErrorText != nil {
		resp["error"] = *j.ErrorText
	}
	writeJSON(w, 200, resp)
}

func (h *Handler) CreateCampaign(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ClientID            string   `json:"client_id"`
		Name                string   `json:"name"`
		Objective           string   `json:"objective"`
		Status              string   `json:"status"`
		SpecialAdCategories []string `json:"special_ad_categories"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, "invalid_json"); return
	}
	if req.ClientID == "" { writeErr(w, 400, "missing_client_id"); return }
	if req.Name == "" { writeErr(w, 400, "missing_name"); return }
	if req.Objective == "" { writeErr(w, 400, "missing_objective"); return }
	if req.Status == "" { req.Status = "PAUSED" }

	out, err := h.Campaigns.CreateCampaign(r.Context(), service.CreateCampaignInput{
		ClientID:            req.ClientID,
		Name:                req.Name,
		Objective:           req.Objective,
		Status:              req.Status,
		SpecialAdCategories: req.SpecialAdCategories,
	})
	if err != nil { writeErr(w, 400, err.Error()); return }
	writeJSON(w, 200, out)
}

func (h *Handler) CreateAdSet(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ClientID         string         `json:"client_id"`
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
	if req.ClientID == "" { writeErr(w, 400, "missing_client_id"); return }
	if req.CampaignID == "" { writeErr(w, 400, "missing_campaign_id"); return }
	if req.Name == "" { writeErr(w, 400, "missing_name"); return }
	if req.BillingEvent == "" { writeErr(w, 400, "missing_billing_event"); return }
	if req.OptimizationGoal == "" { writeErr(w, 400, "missing_optimization_goal"); return }
	if req.DailyBudget == 0 { writeErr(w, 400, "missing_daily_budget"); return }
	if req.Status == "" { req.Status = "PAUSED" }

	out, err := h.AdSets.CreateAdSet(r.Context(), service.CreateAdSetInput{
		ClientID:         req.ClientID,
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
		ClientID   string `json:"client_id"`
		AdSetID    string `json:"adset_id"`
		CreativeID string `json:"creative_id"`
		Name       string `json:"name"`
		Status     string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, "invalid_json"); return
	}
	if req.ClientID == "" { writeErr(w, 400, "missing_client_id"); return }
	if req.AdSetID == "" { writeErr(w, 400, "missing_adset_id"); return }
	if req.CreativeID == "" { writeErr(w, 400, "missing_creative_id"); return }
	if req.Name == "" { writeErr(w, 400, "missing_name"); return }
	if req.Status == "" { req.Status = "PAUSED" }

	out, err := h.Ads.CreateAd(r.Context(), service.CreateAdInput{
		ClientID:   req.ClientID,
		AdSetID:    req.AdSetID,
		CreativeID: req.CreativeID,
		Name:       req.Name,
		Status:     req.Status,
	})
	if err != nil { writeErr(w, 400, err.Error()); return }
	writeJSON(w, 200, out)
}
