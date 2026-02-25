package httpapi

import (
	"net/http"

	"creative-service/internal/auth"
	"creative-service/internal/storage"

	"github.com/go-chi/chi/v5"
)

type Handlers interface {
	Health(http.ResponseWriter, *http.Request)
	GetMe(http.ResponseWriter, *http.Request)
	ListClients(http.ResponseWriter, *http.Request)
	ListAdAccountsByClient(http.ResponseWriter, *http.Request)
	CreateImageCreative(http.ResponseWriter, *http.Request)
	CreateVideoCreative(http.ResponseWriter, *http.Request)
	ListCreatives(http.ResponseWriter, *http.Request)
	GetCreative(http.ResponseWriter, *http.Request)
	SoftDeleteCreative(http.ResponseWriter, *http.Request)
	CreateCampaign(http.ResponseWriter, *http.Request)
	ListCampaigns(http.ResponseWriter, *http.Request)
	UpdateCampaign(http.ResponseWriter, *http.Request)
	DeleteCampaign(http.ResponseWriter, *http.Request)
	CreateAdSet(http.ResponseWriter, *http.Request)
	ListAdSets(http.ResponseWriter, *http.Request)
	UpdateAdSet(http.ResponseWriter, *http.Request)
	DeleteAdSet(http.ResponseWriter, *http.Request)
	CreateAd(http.ResponseWriter, *http.Request)
	ListAds(http.ResponseWriter, *http.Request)
	UpdateAd(http.ResponseWriter, *http.Request)
	DeleteAd(http.ResponseWriter, *http.Request)
	GetBMConfig(http.ResponseWriter, *http.Request)
}

type RouterOptions struct {
	RequireAuth  bool
	AuthVerifier auth.Verifier
	AppUserStore *storage.Store
}

func NewRouter(h Handlers, opts RouterOptions) http.Handler {
	r := chi.NewRouter()
	r.Use(Recoverer, AccessLog, CORS)

	r.Get("/v1/health", h.Health)

	protected := r.With()
	if opts.RequireAuth {
		if opts.AuthVerifier == nil {
			panic("auth is required but auth verifier is nil")
		}
		if opts.AppUserStore == nil {
			panic("auth is required but app user store is nil")
		}
		protected.Use(AuthMiddleware(opts.AuthVerifier))
		protected.Use(EnsureAppUserMiddleware(opts.AppUserStore))
	}

	protected.Get("/v1/me", h.GetMe)
	protected.Get("/v1/clients", h.ListClients)
	protected.Get("/v1/clients/{client_uuid}/ad-accounts", h.ListAdAccountsByClient)

	protected.Post("/v1/creatives/image", h.CreateImageCreative)
	protected.Post("/v1/creatives/video", h.CreateVideoCreative)
	protected.Get("/v1/creatives", h.ListCreatives)
	protected.Get("/v1/creatives/{creative_id}", h.GetCreative)
	protected.Delete("/v1/creatives/{creative_id}", h.SoftDeleteCreative)

	protected.Post("/v1/campaigns", h.CreateCampaign)
	protected.Get("/v1/campaigns", h.ListCampaigns)
	protected.Patch("/v1/campaigns/{campaign_id}", h.UpdateCampaign)
	protected.Delete("/v1/campaigns/{campaign_id}", h.DeleteCampaign)

	protected.Post("/v1/adsets", h.CreateAdSet)
	protected.Get("/v1/adsets", h.ListAdSets)
	protected.Patch("/v1/adsets/{adset_id}", h.UpdateAdSet)
	protected.Delete("/v1/adsets/{adset_id}", h.DeleteAdSet)

	protected.Post("/v1/ads", h.CreateAd)
	protected.Get("/v1/ads", h.ListAds)
	protected.Patch("/v1/ads/{ad_id}", h.UpdateAd)
	protected.Delete("/v1/ads/{ad_id}", h.DeleteAd)

	protected.Get("/v1/bms/{bm_uuid}/config", h.GetBMConfig)

	return r
}
