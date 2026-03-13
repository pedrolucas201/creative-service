package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"net/mail"
	"strconv"
	"strings"

	"creative-service/internal/auth"
	"creative-service/internal/bm"
	"creative-service/internal/service"
	"creative-service/internal/storage"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	CreativeSync                    *service.CreativeSyncService
	Store                           *storage.Store
	Campaigns                       *service.CampaignService
	AdSets                          *service.AdSetService
	Ads                             *service.AdService
	BM                              *bm.Service
	UserManager                     auth.UserManager
	MetaWebhookVerifyToken          string
	MetaWebhookAppSecret            string
	ContingencyTasks                ContingencyTaskDispatcher
	ContingencyInternalToken        string
	ContingencyMonitorAdAccounts    []string
	ContingencyDefaultMaxCandidates int
	ContingencyDefaultMaxAttempts   int
	ContingencyDefaultRefreshStatus bool
	ContingencyDispatchViaTasks     bool
}

type ContingencyTaskDispatcher interface {
	EnqueueContingencyExecution(ctx context.Context, incidentUUID string, maxAttempts int) (string, error)
}

var (
	adAccountWriteRoles = map[string]struct{}{
		"owner":    {},
		"admin":    {},
		"operator": {},
	}
	contingencyManualRoles = map[string]struct{}{
		"owner": {},
		"admin": {},
	}
	bmConfigReadRoles = map[string]struct{}{
		"owner": {},
		"admin": {},
	}
	bmAdminRoles = map[string]struct{}{
		"owner": {},
		"admin": {},
	}
	assignableUserRoles = map[string]struct{}{
		"owner":    {},
		"admin":    {},
		"operator": {},
		"viewer":   {},
	}
	allowedPasswordLengths = map[int]struct{}{
		8:  {},
		10: {},
	}
	allowedStatusEntityTypes = map[string]struct{}{
		"creative": {},
		"campaign": {},
		"adset":    {},
		"ad":       {},
	}
	defaultStatusEntityTypes = []string{"creative", "campaign", "adset", "ad"}
)

const (
	defaultPasswordLength = 10
	passwordCharset       = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789!@#$%&*"
	defaultCreativeStatus = "NOT_LINKED"
)

func roleAllowed(role string, allowed map[string]struct{}) bool {
	_, ok := allowed[strings.ToLower(strings.TrimSpace(role))]
	return ok
}

func normalizeRole(role string) string {
	return strings.ToLower(strings.TrimSpace(role))
}

func normalizeStatusEntityType(entityType string) string {
	return strings.ToLower(strings.TrimSpace(entityType))
}

func parseStatusEntityTypes(raw []string) ([]string, bool) {
	if len(raw) == 0 {
		out := append([]string(nil), defaultStatusEntityTypes...)
		return out, false
	}

	out := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	invalid := false

	for _, v := range raw {
		for _, part := range strings.Split(v, ",") {
			entityType := normalizeStatusEntityType(part)
			if entityType == "" {
				continue
			}
			if _, ok := allowedStatusEntityTypes[entityType]; !ok {
				invalid = true
				continue
			}
			if _, ok := seen[entityType]; ok {
				continue
			}
			seen[entityType] = struct{}{}
			out = append(out, entityType)
		}
	}

	if len(out) == 0 {
		if invalid {
			return nil, true
		}
		out = append([]string(nil), defaultStatusEntityTypes...)
		return out, false
	}

	return out, invalid
}

func hasStatusEntityType(entityTypes []string, wanted string) bool {
	for _, entityType := range entityTypes {
		if entityType == wanted {
			return true
		}
	}
	return false
}

func parseBoolDefault(raw string, def bool) bool {
	v := strings.TrimSpace(raw)
	if v == "" {
		return def
	}
	parsed, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return parsed
}

func resolvePasswordLength(raw int) (int, bool) {
	if raw == 0 {
		return defaultPasswordLength, true
	}
	_, ok := allowedPasswordLengths[raw]
	return raw, ok
}

func generateRandomPassword(length int) (string, error) {
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	charsetLen := len(passwordCharset)
	for i := range buf {
		buf[i] = passwordCharset[int(buf[i])%charsetLen]
	}

	return string(buf), nil
}

func (h *Handler) requireAdAccountAccess(w http.ResponseWriter, r *http.Request, adAccountID string) bool {
	return h.requireAdAccountAccessWithRoles(w, r, adAccountID, nil)
}

func (h *Handler) requireAdAccountWriteAccess(w http.ResponseWriter, r *http.Request, adAccountID string) bool {
	return h.requireAdAccountAccessWithRoles(w, r, adAccountID, adAccountWriteRoles)
}

func (h *Handler) requireAdAccountAccessWithRoles(
	w http.ResponseWriter,
	r *http.Request,
	adAccountID string,
	requiredRoles map[string]struct{},
) bool {
	if adAccountID == "" {
		writeErr(w, http.StatusBadRequest, "missing_ad_account_id")
		return false
	}

	identity, ok := auth.IdentityFromContext(r.Context())
	if !ok || identity == nil || identity.UID == "" {
		writeErr(w, http.StatusUnauthorized, "missing_identity")
		return false
	}

	if err := h.Store.EnsureAppUser(r.Context(), identity.UID, identity.Email); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed_to_sync_user")
		return false
	}

	role, allowed, err := h.Store.UserRoleForAdAccount(r.Context(), identity.UID, adAccountID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed_to_check_access")
		return false
	}
	if !allowed {
		writeErr(w, http.StatusForbidden, "forbidden_for_ad_account")
		return false
	}
	if requiredRoles != nil && !roleAllowed(role, requiredRoles) {
		writeErr(w, http.StatusForbidden, "insufficient_role_for_ad_account")
		return false
	}

	return true
}

func (h *Handler) requireBMAccessWithRoles(
	w http.ResponseWriter,
	r *http.Request,
	bmUUID string,
	requiredRoles map[string]struct{},
) bool {
	if bmUUID == "" {
		writeErr(w, http.StatusBadRequest, "missing_bm_uuid")
		return false
	}

	identity, ok := auth.IdentityFromContext(r.Context())
	if !ok || identity == nil || identity.UID == "" {
		writeErr(w, http.StatusUnauthorized, "missing_identity")
		return false
	}

	if err := h.Store.EnsureAppUser(r.Context(), identity.UID, identity.Email); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed_to_sync_user")
		return false
	}

	role, allowed, err := h.Store.UserRoleForBM(r.Context(), identity.UID, bmUUID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed_to_check_bm_access")
		return false
	}
	if !allowed {
		writeErr(w, http.StatusForbidden, "forbidden_for_bm")
		return false
	}
	if requiredRoles != nil && !roleAllowed(role, requiredRoles) {
		writeErr(w, http.StatusForbidden, "insufficient_role_for_bm")
		return false
	}

	return true
}

func (h *Handler) GetMe(w http.ResponseWriter, r *http.Request) {
	identity, ok := auth.IdentityFromContext(r.Context())
	if !ok || identity == nil || identity.UID == "" {
		writeErr(w, http.StatusUnauthorized, "missing_identity")
		return
	}

	if err := h.Store.EnsureAppUser(r.Context(), identity.UID, identity.Email); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed_to_sync_user")
		return
	}

	writeJSON(w, 200, map[string]any{
		"uid":   identity.UID,
		"email": identity.Email,
	})
}

func (h *Handler) CreateManagedUser(w http.ResponseWriter, r *http.Request) {
	if h.UserManager == nil {
		writeErr(w, http.StatusInternalServerError, "firebase_user_management_unavailable")
		return
	}

	var req struct {
		Email          string `json:"email"`
		BMUUID         string `json:"bm_uuid"`
		Role           string `json:"role"`
		PasswordLength int    `json:"password_length"`
		IsActive       *bool  `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_json")
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" {
		writeErr(w, http.StatusBadRequest, "missing_email")
		return
	}
	if _, err := mail.ParseAddress(req.Email); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_email")
		return
	}

	req.BMUUID = strings.TrimSpace(req.BMUUID)
	if req.BMUUID == "" {
		writeErr(w, http.StatusBadRequest, "missing_bm_uuid")
		return
	}
	if !h.requireBMAccessWithRoles(w, r, req.BMUUID, bmAdminRoles) {
		return
	}

	req.Role = normalizeRole(req.Role)
	if req.Role == "" {
		req.Role = "viewer"
	}
	if !roleAllowed(req.Role, assignableUserRoles) {
		writeErr(w, http.StatusBadRequest, "invalid_role")
		return
	}

	passwordLength, ok := resolvePasswordLength(req.PasswordLength)
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid_password_length")
		return
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	tempPassword, err := generateRandomPassword(passwordLength)
	if err != nil {
		writeErrCause(w, http.StatusInternalServerError, "failed_to_generate_password", err)
		return
	}

	uid, created, err := h.UserManager.CreateOrUpdateUserPassword(r.Context(), req.Email, tempPassword)
	if err != nil {
		writeErrCause(w, http.StatusInternalServerError, "failed_to_create_firebase_user", err)
		return
	}

	if err := h.Store.EnsureAppUser(r.Context(), uid, req.Email); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed_to_sync_user")
		return
	}

	if err := h.Store.UpsertUserBMAccess(r.Context(), uid, req.BMUUID, req.Role, isActive); err != nil {
		writeErrCause(w, http.StatusInternalServerError, "failed_to_bind_user_bm_access", err)
		return
	}

	status := http.StatusCreated
	if !created {
		status = http.StatusOK
	}

	writeJSON(w, status, map[string]any{
		"uid":                   uid,
		"email":                 req.Email,
		"bm_uuid":               req.BMUUID,
		"role":                  req.Role,
		"is_active":             isActive,
		"temporary_password":    tempPassword,
		"firebase_user_created": created,
	})
}

// GetBMConfig implements [Handlers].
func (h *Handler) GetBMConfig(w http.ResponseWriter, r *http.Request) {
	bmUUID := chi.URLParam(r, "bm_uuid")
	if !h.requireBMAccessWithRoles(w, r, bmUUID, bmConfigReadRoles) {
		return
	}

	cfg, err := h.BM.GetBMConfig(r.Context(), bmUUID)
	if err != nil {
		writeErrCause(w, 400, "failed_to_get_bm_config", err)
		return
	}

	writeJSON(w, 200, cfg)
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (h *Handler) ListClients(w http.ResponseWriter, r *http.Request) {
	identity, hasIdentity := auth.IdentityFromContext(r.Context())

	var (
		clients []storage.Client
		err     error
	)
	if hasIdentity && identity != nil && identity.UID != "" {
		clients, err = h.Store.ListClientsByUID(r.Context(), identity.UID)
	} else {
		clients, err = h.Store.ListClients(r.Context())
	}
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
		writeErr(w, 400, "invalid_multipart")
		return
	}

	adAccountID := r.FormValue("ad_account_id")
	if adAccountID == "" {
		writeErr(w, 400, "missing_ad_account_id")
		return
	}
	if !h.requireAdAccountWriteAccess(w, r, adAccountID) {
		return
	}

	file, hdr, err := r.FormFile("image")
	if err != nil {
		writeErr(w, 400, "missing_image")
		return
	}
	defer file.Close()
	b, _ := io.ReadAll(file)

	out, err := h.CreativeSync.CreateImageCreative(r.Context(), service.ImageCreativeInput{
		AdAccountID: adAccountID,
		Name:        r.FormValue("name"),
		Link:        r.FormValue("link"),
		Message:     r.FormValue("message"),
		Headline:    r.FormValue("headline"),
		Description: r.FormValue("description"),
		ImageName:   hdr.Filename,
		ImageBytes:  b,
	})
	if err != nil {
		writeErrCause(w, 400, "failed_to_create_image_creative", err)
		return
	}
	writeJSON(w, 200, out)
}

func (h *Handler) CreateVideoCreative(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(1024 << 20); err != nil {
		writeErr(w, 400, "invalid_multipart")
		return
	}

	adAccountID := r.FormValue("ad_account_id")
	if adAccountID == "" {
		writeErr(w, 400, "missing_ad_account_id")
		return
	}
	if !h.requireAdAccountWriteAccess(w, r, adAccountID) {
		return
	}

	videoFile, videoHeader, err := r.FormFile("video")
	if err != nil {
		writeErr(w, 400, "missing_video")
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
		AdAccountID: adAccountID,
		Name:        r.FormValue("name"),
		Link:        r.FormValue("link"),
		Message:     r.FormValue("message"),
		Headline:    r.FormValue("headline"),
		Description: r.FormValue("description"),
		VideoName:   videoHeader.Filename,
		VideoBytes:  videoBytes,
		ThumbName:   thumbHeader.Filename,
		ThumbBytes:  thumbBytes,
	})
	if err != nil {
		writeErrCause(w, 400, "failed_to_create_video_creative", err)
		return
	}

	writeJSON(w, 200, out)
}

func (h *Handler) CreateCampaign(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AdAccountID                 string   `json:"ad_account_id"`
		Name                        string   `json:"name"`
		Objective                   string   `json:"objective"`
		Status                      string   `json:"status"`
		SpecialAdCategories         []string `json:"special_ad_categories"`
		BuyingType                  string   `json:"buying_type"`
		IsAdSetBudgetSharingEnabled bool     `json:"is_adset_budget_sharing_enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, "invalid_json")
		return
	}

	if req.AdAccountID == "" {
		writeErr(w, 400, "missing_ad_account_id")
		return
	}
	if !h.requireAdAccountWriteAccess(w, r, req.AdAccountID) {
		return
	}
	if req.Name == "" {
		writeErr(w, 400, "missing_name")
		return
	}
	if req.Objective == "" {
		writeErr(w, 400, "missing_objective")
		return
	}
	if req.Status == "" {
		req.Status = "PAUSED"
	}
	if req.SpecialAdCategories == nil {
		req.SpecialAdCategories = []string{}
	}
	if req.BuyingType == "" {
		req.BuyingType = "AUCTION"
	}

	out, err := h.Campaigns.CreateCampaign(r.Context(), service.CreateCampaignInput{
		AdAccountID:                 req.AdAccountID,
		Name:                        req.Name,
		Objective:                   req.Objective,
		Status:                      req.Status,
		SpecialAdCategories:         req.SpecialAdCategories,
		BuyingType:                  req.BuyingType,
		IsAdSetBudgetSharingEnabled: req.IsAdSetBudgetSharingEnabled,
	})
	if err != nil {
		writeErrCause(w, 400, "failed_to_create_campaign", err)
		return
	}
	writeJSON(w, 200, out)
}

func (h *Handler) CreateAdSet(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AdAccountID      string         `json:"ad_account_id"`
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
		writeErr(w, 400, "invalid_json")
		return
	}

	if req.AdAccountID == "" {
		writeErr(w, 400, "missing_ad_account_id")
		return
	}
	if !h.requireAdAccountWriteAccess(w, r, req.AdAccountID) {
		return
	}
	if req.CampaignID == "" {
		writeErr(w, 400, "missing_campaign_id")
		return
	}
	if req.Name == "" {
		writeErr(w, 400, "missing_name")
		return
	}
	if req.BillingEvent == "" {
		writeErr(w, 400, "missing_billing_event")
		return
	}
	if req.OptimizationGoal == "" {
		writeErr(w, 400, "missing_optimization_goal")
		return
	}
	if req.DailyBudget == 0 {
		writeErr(w, 400, "missing_daily_budget")
		return
	}
	if req.Status == "" {
		req.Status = "PAUSED"
	}

	out, err := h.AdSets.CreateAdSet(r.Context(), service.CreateAdSetInput{
		AdAccountID:      req.AdAccountID,
		CampaignID:       req.CampaignID,
		Name:             req.Name,
		BillingEvent:     req.BillingEvent,
		OptimizationGoal: req.OptimizationGoal,
		BidAmount:        req.BidAmount,
		DailyBudget:      req.DailyBudget,
		Targeting:        req.Targeting,
		Status:           req.Status,
	})
	if err != nil {
		writeErrCause(w, 400, "failed_to_create_adset", err)
		return
	}
	writeJSON(w, 200, out)
}

func (h *Handler) CreateAd(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AdAccountID string `json:"ad_account_id"`
		AdSetID     string `json:"adset_id"`
		CreativeID  string `json:"creative_id"`
		Name        string `json:"name"`
		Status      string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, "invalid_json")
		return
	}

	if req.AdAccountID == "" {
		writeErr(w, 400, "missing_ad_account_id")
		return
	}
	if !h.requireAdAccountWriteAccess(w, r, req.AdAccountID) {
		return
	}
	if req.AdSetID == "" {
		writeErr(w, 400, "missing_adset_id")
		return
	}
	if req.CreativeID == "" {
		writeErr(w, 400, "missing_creative_id")
		return
	}
	if req.Name == "" {
		writeErr(w, 400, "missing_name")
		return
	}
	if req.Status == "" {
		req.Status = "PAUSED"
	}

	out, err := h.Ads.CreateAd(r.Context(), service.CreateAdInput{
		AdAccountID: req.AdAccountID,
		AdSetID:     req.AdSetID,
		CreativeID:  req.CreativeID,
		Name:        req.Name,
		Status:      req.Status,
	})
	if err != nil {
		writeErrCause(w, 400, "failed_to_create_ad", err)
		return
	}
	writeJSON(w, 200, out)
}

func (h *Handler) ListCreatives(w http.ResponseWriter, r *http.Request) {
	adAccountID := r.URL.Query().Get("ad_account_id")
	typeFilter := r.URL.Query().Get("type")
	if !h.requireAdAccountAccess(w, r, adAccountID) {
		return
	}

	if typeFilter != "" && typeFilter != "image" && typeFilter != "video" {
		writeErr(w, 400, "invalid_type_filter")
		return
	}

	creatives, err := h.Store.ListCreatives(r.Context(), adAccountID, typeFilter)
	if err != nil {
		writeErr(w, 500, "failed to list creatives")
		return
	}

	statusByCreativeID, err := h.Store.ListEntityStatusMap(r.Context(), adAccountID, "creative")
	if err != nil {
		writeErrCause(w, 500, "failed_to_list_statuses", err)
		return
	}

	type creativeWithStatus struct {
		storage.Creative
		Status string `json:"status"`
	}
	creativeRows := make([]creativeWithStatus, 0, len(creatives))
	for _, creative := range creatives {
		status := defaultCreativeStatus
		if rec, ok := statusByCreativeID[creative.CreativeID]; ok && strings.TrimSpace(rec.Status) != "" {
			status = rec.Status
		}

		creativeRows = append(creativeRows, creativeWithStatus{
			Creative: creative,
			Status:   status,
		})
	}

	writeJSON(w, 200, map[string]any{
		"creatives": creativeRows,
		"count":     len(creativeRows),
	})
}

func (h *Handler) GetCreative(w http.ResponseWriter, r *http.Request) {
	creativeID := chi.URLParam(r, "creative_id")
	if creativeID == "" {
		writeErr(w, 400, "missing_creative_id")
		return
	}

	creative, err := h.Store.GetCreative(r.Context(), creativeID)
	if err != nil {
		writeErr(w, 404, "creative_not_found")
		return
	}
	if !h.requireAdAccountAccess(w, r, creative.AdAccountID) {
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

	identity, hasIdentity := auth.IdentityFromContext(r.Context())

	var (
		adAccounts []storage.AdAccount
		err        error
	)
	if hasIdentity && identity != nil && identity.UID != "" {
		adAccounts, err = h.Store.ListAdAccountsByClientForUID(r.Context(), clientUUID, identity.UID)
	} else {
		adAccounts, err = h.Store.ListAdAccountsByClient(r.Context(), clientUUID)
	}
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

	creative, err := h.Store.GetCreative(r.Context(), creativeID)
	if err != nil {
		writeErr(w, 404, "creative_not_found")
		return
	}
	if !h.requireAdAccountWriteAccess(w, r, creative.AdAccountID) {
		return
	}

	err = h.Store.SoftDeleteCreative(r.Context(), creativeID)
	if err != nil {
		writeErrCause(w, 404, "failed_to_delete_creative", err)
		return
	}

	writeJSON(w, 200, map[string]any{
		"message":     "creative deleted successfully",
		"creative_id": creativeID,
	})
}

func (h *Handler) ListCampaigns(w http.ResponseWriter, r *http.Request) {
	adAccountID := r.URL.Query().Get("ad_account_id")
	if adAccountID == "" {
		writeErr(w, 400, "missing_ad_account_id")
		return
	}
	if !h.requireAdAccountAccess(w, r, adAccountID) {
		return
	}

	out, err := h.Campaigns.ListCampaigns(r.Context(), service.ListCampaignsInput{
		AdAccountID: adAccountID,
	})
	if err != nil {
		writeErrCause(w, 500, "failed_to_list_campaigns", err)
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
	if !h.requireAdAccountAccess(w, r, adAccountID) {
		return
	}

	out, err := h.AdSets.ListAdSets(r.Context(), service.ListAdSetsInput{
		AdAccountID: adAccountID,
	})
	if err != nil {
		writeErrCause(w, 500, "failed_to_list_adsets", err)
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
	if !h.requireAdAccountAccess(w, r, adAccountID) {
		return
	}

	out, err := h.Ads.ListAds(r.Context(), service.ListAdsInput{
		AdAccountID: adAccountID,
	})
	if err != nil {
		writeErrCause(w, 500, "failed_to_list_ads", err)
		return
	}

	writeJSON(w, 200, out)
}

func (h *Handler) syncStatusCache(
	ctx context.Context,
	adAccountID string,
	entityTypes []string,
) (map[string]int, map[string]string) {
	synced := make(map[string]int, len(entityTypes))
	syncErrors := map[string]string{}

	if hasStatusEntityType(entityTypes, "campaign") {
		out, err := h.Campaigns.ListCampaigns(ctx, service.ListCampaignsInput{
			AdAccountID: adAccountID,
		})
		if err != nil {
			syncErrors["campaign"] = err.Error()
		} else {
			synced["campaign"] = len(out.Campaigns)
		}
	}

	if hasStatusEntityType(entityTypes, "adset") {
		out, err := h.AdSets.ListAdSets(ctx, service.ListAdSetsInput{
			AdAccountID: adAccountID,
		})
		if err != nil {
			syncErrors["adset"] = err.Error()
		} else {
			synced["adset"] = len(out.AdSets)
		}
	}

	needAdsLookup := hasStatusEntityType(entityTypes, "ad") || hasStatusEntityType(entityTypes, "creative")
	if needAdsLookup {
		out, err := h.Ads.ListAds(ctx, service.ListAdsInput{
			AdAccountID: adAccountID,
		})
		if err != nil {
			if hasStatusEntityType(entityTypes, "ad") {
				syncErrors["ad"] = err.Error()
			}
			if hasStatusEntityType(entityTypes, "creative") {
				syncErrors["creative"] = err.Error()
			}
		} else {
			if hasStatusEntityType(entityTypes, "ad") {
				synced["ad"] = len(out.Ads)
			}
			if hasStatusEntityType(entityTypes, "creative") {
				creativeRows, err := h.Store.ListEntityStatuses(ctx, adAccountID, "creative")
				if err != nil {
					syncErrors["creative"] = err.Error()
				} else {
					synced["creative"] = len(creativeRows)
				}
			}
		}
	}

	return synced, syncErrors
}

func (h *Handler) loadStatusSnapshot(
	ctx context.Context,
	adAccountID string,
	entityTypes []string,
) (map[string][]statusRecordView, error) {
	out := make(map[string][]statusRecordView, len(entityTypes))
	for _, entityType := range entityTypes {
		rows, err := h.Store.ListEntityStatuses(ctx, adAccountID, entityType)
		if err != nil {
			return nil, err
		}
		out[entityType] = buildStatusViews(rows)
	}
	return out, nil
}

func (h *Handler) SyncStatusCache(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AdAccountID string   `json:"ad_account_id"`
		EntityTypes []string `json:"entity_types"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		writeErr(w, 400, "invalid_json")
		return
	}

	req.AdAccountID = strings.TrimSpace(req.AdAccountID)
	if req.AdAccountID == "" {
		writeErr(w, 400, "missing_ad_account_id")
		return
	}

	entityTypes, hasInvalid := parseStatusEntityTypes(req.EntityTypes)
	if hasInvalid || len(entityTypes) == 0 {
		writeErr(w, 400, "invalid_entity_type")
		return
	}

	if !h.requireAdAccountAccess(w, r, req.AdAccountID) {
		return
	}

	synced, syncErrors := h.syncStatusCache(r.Context(), req.AdAccountID, entityTypes)
	if len(syncErrors) == len(entityTypes) {
		writeErr(w, 500, "failed_to_sync_statuses")
		return
	}

	statuses, err := h.loadStatusSnapshot(r.Context(), req.AdAccountID, entityTypes)
	if err != nil {
		writeErrCause(w, 500, "failed_to_list_statuses", err)
		return
	}

	resp := map[string]any{
		"ad_account_id": req.AdAccountID,
		"entity_types":  entityTypes,
		"synced":        synced,
		"statuses":      statuses,
	}
	if len(syncErrors) > 0 {
		resp["sync_errors"] = syncErrors
	}

	statusCode := http.StatusOK
	if len(syncErrors) > 0 {
		statusCode = http.StatusMultiStatus
	}
	writeJSON(w, statusCode, resp)
}

func (h *Handler) ListStatusCache(w http.ResponseWriter, r *http.Request) {
	adAccountID := strings.TrimSpace(r.URL.Query().Get("ad_account_id"))
	if adAccountID == "" {
		writeErr(w, 400, "missing_ad_account_id")
		return
	}

	rawEntityTypes := append([]string{}, r.URL.Query()["entity_type"]...)
	rawEntityTypes = append(rawEntityTypes, r.URL.Query()["entity_types"]...)

	entityTypes, hasInvalid := parseStatusEntityTypes(rawEntityTypes)
	if hasInvalid || len(entityTypes) == 0 {
		writeErr(w, 400, "invalid_entity_type")
		return
	}

	if !h.requireAdAccountAccess(w, r, adAccountID) {
		return
	}

	refresh := parseBoolDefault(r.URL.Query().Get("refresh"), false)
	var (
		synced     map[string]int
		syncErrors map[string]string
	)
	if refresh {
		synced, syncErrors = h.syncStatusCache(r.Context(), adAccountID, entityTypes)
		if len(syncErrors) == len(entityTypes) {
			writeErr(w, 500, "failed_to_sync_statuses")
			return
		}
	}

	statuses, err := h.loadStatusSnapshot(r.Context(), adAccountID, entityTypes)
	if err != nil {
		writeErrCause(w, 500, "failed_to_list_statuses", err)
		return
	}

	resp := map[string]any{
		"ad_account_id": adAccountID,
		"entity_types":  entityTypes,
		"statuses":      statuses,
	}
	if refresh {
		resp["synced"] = synced
	}
	if len(syncErrors) > 0 {
		resp["sync_errors"] = syncErrors
	}

	statusCode := http.StatusOK
	if len(syncErrors) > 0 {
		statusCode = http.StatusMultiStatus
	}
	writeJSON(w, statusCode, resp)
}

// ======= UPDATE Campaign =======

func (h *Handler) UpdateCampaign(w http.ResponseWriter, r *http.Request) {
	campaignID := chi.URLParam(r, "campaign_id")
	if campaignID == "" {
		writeErr(w, 400, "missing_campaign_id")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeErr(w, 400, "invalid_body")
		return
	}

	var req struct {
		AdAccountID string  `json:"ad_account_id"`
		Name        *string `json:"name"`
		Status      *string `json:"status"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeErr(w, 400, "invalid_json")
		return
	}

	if req.AdAccountID == "" {
		writeErr(w, 400, "missing_ad_account_id")
		return
	}
	if !h.requireAdAccountWriteAccess(w, r, req.AdAccountID) {
		return
	}

	if err := h.Campaigns.UpdateCampaign(r.Context(), service.UpdateCampaignInput{
		AdAccountID: req.AdAccountID,
		CampaignID:  campaignID,
		Name:        req.Name,
		Status:      req.Status,
	}); err != nil {
		writeErrCause(w, 500, "failed_to_update_campaign", err)
		return
	}

	writeJSON(w, 200, map[string]any{"success": true})
}

// ======= DELETE Campaign (soft delete) =======

func (h *Handler) DeleteCampaign(w http.ResponseWriter, r *http.Request) {
	campaignID := chi.URLParam(r, "campaign_id")
	if campaignID == "" {
		writeErr(w, 400, "missing_campaign_id")
		return
	}

	adAccountID := r.URL.Query().Get("ad_account_id")
	if adAccountID == "" {
		writeErr(w, 400, "missing_ad_account_id")
		return
	}
	if !h.requireAdAccountWriteAccess(w, r, adAccountID) {
		return
	}

	if err := h.Campaigns.DeleteCampaign(r.Context(), service.DeleteCampaignInput{
		AdAccountID: adAccountID,
		CampaignID:  campaignID,
	}); err != nil {
		writeErrCause(w, 500, "failed_to_delete_campaign", err)
		return
	}

	writeJSON(w, 200, map[string]any{"success": true})
}

// ======= UPDATE AdSet =======

func (h *Handler) UpdateAdSet(w http.ResponseWriter, r *http.Request) {
	adsetID := chi.URLParam(r, "adset_id")
	if adsetID == "" {
		writeErr(w, 400, "missing_adset_id")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeErr(w, 400, "invalid_body")
		return
	}

	var req struct {
		AdAccountID string  `json:"ad_account_id"`
		Name        *string `json:"name"`
		Status      *string `json:"status"`
		DailyBudget *int    `json:"daily_budget"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeErr(w, 400, "invalid_json")
		return
	}

	if req.AdAccountID == "" {
		writeErr(w, 400, "missing_ad_account_id")
		return
	}
	if !h.requireAdAccountWriteAccess(w, r, req.AdAccountID) {
		return
	}

	if err := h.AdSets.UpdateAdSet(r.Context(), service.UpdateAdSetInput{
		AdAccountID: req.AdAccountID,
		AdSetID:     adsetID,
		Name:        req.Name,
		Status:      req.Status,
		DailyBudget: req.DailyBudget,
	}); err != nil {
		writeErrCause(w, 500, "failed_to_update_adset", err)
		return
	}

	writeJSON(w, 200, map[string]any{"success": true})
}

// ======= DELETE AdSet (soft delete) =======

func (h *Handler) DeleteAdSet(w http.ResponseWriter, r *http.Request) {
	adsetID := chi.URLParam(r, "adset_id")
	if adsetID == "" {
		writeErr(w, 400, "missing_adset_id")
		return
	}

	adAccountID := r.URL.Query().Get("ad_account_id")
	if adAccountID == "" {
		writeErr(w, 400, "missing_ad_account_id")
		return
	}
	if !h.requireAdAccountWriteAccess(w, r, adAccountID) {
		return
	}

	if err := h.AdSets.DeleteAdSet(r.Context(), service.DeleteAdSetInput{
		AdAccountID: adAccountID,
		AdSetID:     adsetID,
	}); err != nil {
		writeErrCause(w, 500, "failed_to_delete_adset", err)
		return
	}

	writeJSON(w, 200, map[string]any{"success": true})
}

// ======= UPDATE Ad =======

func (h *Handler) UpdateAd(w http.ResponseWriter, r *http.Request) {
	adID := chi.URLParam(r, "ad_id")
	if adID == "" {
		writeErr(w, 400, "missing_ad_id")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeErr(w, 400, "invalid_body")
		return
	}

	var req struct {
		AdAccountID string  `json:"ad_account_id"`
		Name        *string `json:"name"`
		Status      *string `json:"status"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeErr(w, 400, "invalid_json")
		return
	}

	if req.AdAccountID == "" {
		writeErr(w, 400, "missing_ad_account_id")
		return
	}
	if !h.requireAdAccountWriteAccess(w, r, req.AdAccountID) {
		return
	}

	if err := h.Ads.UpdateAd(r.Context(), service.UpdateAdInput{
		AdAccountID: req.AdAccountID,
		AdID:        adID,
		Name:        req.Name,
		Status:      req.Status,
	}); err != nil {
		writeErrCause(w, 500, "failed_to_update_ad", err)
		return
	}

	writeJSON(w, 200, map[string]any{"success": true})
}

// ======= DELETE Ad (soft delete) =======

func (h *Handler) DeleteAd(w http.ResponseWriter, r *http.Request) {
	adID := chi.URLParam(r, "ad_id")
	if adID == "" {
		writeErr(w, 400, "missing_ad_id")
		return
	}

	adAccountID := r.URL.Query().Get("ad_account_id")
	if adAccountID == "" {
		writeErr(w, 400, "missing_ad_account_id")
		return
	}
	if !h.requireAdAccountWriteAccess(w, r, adAccountID) {
		return
	}

	if err := h.Ads.DeleteAd(r.Context(), service.DeleteAdInput{
		AdAccountID: adAccountID,
		AdID:        adID,
	}); err != nil {
		writeErrCause(w, 500, "failed_to_delete_ad", err)
		return
	}

	writeJSON(w, 200, map[string]any{"success": true})
}
