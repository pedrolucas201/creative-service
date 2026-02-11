package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Handlers interface {
	Health(http.ResponseWriter, *http.Request)
	ListClients(http.ResponseWriter, *http.Request)
	ListAdAccountsByClient(http.ResponseWriter, *http.Request)
	CreateImageCreative(http.ResponseWriter, *http.Request)
	CreateVideoCreative(http.ResponseWriter, *http.Request)
	ListCreatives(http.ResponseWriter, *http.Request)
	GetCreative(http.ResponseWriter, *http.Request)
	SoftDeleteCreative(http.ResponseWriter, *http.Request)
	CreateCampaign(http.ResponseWriter, *http.Request)
	ListCampaigns(http.ResponseWriter, *http.Request)
	CreateAdSet(http.ResponseWriter, *http.Request)
	ListAdSets(http.ResponseWriter, *http.Request)
	CreateAd(http.ResponseWriter, *http.Request)
	ListAds(http.ResponseWriter, *http.Request)
}

func NewRouter(h Handlers) http.Handler {
	r := chi.NewRouter()
	r.Use(Recoverer, AccessLog)

	r.Get("/v1/health", h.Health)
	
	// Clients & Ad Accounts
	r.Get("/v1/clients", h.ListClients)
	r.Get("/v1/clients/{client_uuid}/ad-accounts", h.ListAdAccountsByClient)
	
	// Creatives
	r.Post("/v1/creatives/image", h.CreateImageCreative)
	r.Post("/v1/creatives/video", h.CreateVideoCreative)
	r.Get("/v1/creatives", h.ListCreatives)
	r.Get("/v1/creatives/{creative_id}", h.GetCreative)
	r.Delete("/v1/creatives/{creative_id}", h.SoftDeleteCreative)
	
	// Campaigns, AdSets, Ads
	r.Post("/v1/campaigns", h.CreateCampaign)
	r.Get("/v1/campaigns", h.ListCampaigns)
	r.Post("/v1/adsets", h.CreateAdSet)
	r.Get("/v1/adsets", h.ListAdSets)
	r.Post("/v1/ads", h.CreateAd)
	r.Get("/v1/ads", h.ListAds)

	return r
}
