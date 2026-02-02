package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Handlers interface {
	Health(http.ResponseWriter, *http.Request)
	CreateImageCreative(http.ResponseWriter, *http.Request)
	CreateVideoCreativeJob(http.ResponseWriter, *http.Request)
	GetJob(http.ResponseWriter, *http.Request)
	CreateCampaign(http.ResponseWriter, *http.Request)
	CreateAdSet(http.ResponseWriter, *http.Request)
	CreateAd(http.ResponseWriter, *http.Request)
}

func NewRouter(h Handlers) http.Handler {
	r := chi.NewRouter()
	r.Use(Recoverer, AccessLog)

	r.Get("/v1/health", h.Health)
	r.Post("/v1/creatives/image", h.CreateImageCreative)
	r.Post("/v1/jobs/creatives/video", h.CreateVideoCreativeJob)
	r.Get("/v1/jobs/{job_id}", h.GetJob)
	r.Post("/v1/campaigns", h.CreateCampaign)
	r.Post("/v1/adsets", h.CreateAdSet)
	r.Post("/v1/ads", h.CreateAd)

	return r
}
