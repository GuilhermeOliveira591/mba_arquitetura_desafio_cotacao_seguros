package quotation

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/GuilhermeOliveira591/mba_arquitetura_desafio_cotacao_seguros/internal/partner"
	"github.com/GuilhermeOliveira591/mba_arquitetura_desafio_cotacao_seguros/internal/platform"
)

// TenantHeader identifies the broker asking for the quote.
const TenantHeader = "X-Tenant-Id"

// requestLimit cuts off absurd bodies before they turn into memory.
const requestLimit = 1 << 20 // 1 MiB

// API exposes the HTTP contract of the quotation-api.
type API struct {
	service *Service
	tenants map[string]bool
}

func NewAPI(service *Service, tenants []string) *API {
	known := make(map[string]bool, len(tenants))
	for _, t := range tenants {
		known[t] = true
	}
	return &API{service: service, tenants: known}
}

// Routes builds the API router.
//
//	POST /quotes   aggregated quote from the partners (requires X-Tenant-Id)
//	GET  /healthz  process health
func (a *API) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /quotes", a.quote)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		platform.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	return mux
}

func (a *API) quote(w http.ResponseWriter, r *http.Request) {
	tenant := r.Header.Get(TenantHeader)
	if tenant == "" {
		platform.WriteError(w, http.StatusBadRequest, TenantHeader+" is required")
		return
	}
	// An unknown broker is not "unauthenticated", it is "not yours". Isolation between brokers is a
	// requirement of the scenario (SUSEP and LGPD), not an implementation detail.
	if !a.tenants[tenant] {
		platform.WriteError(w, http.StatusForbidden, "broker not enabled on this platform")
		return
	}

	var request Request
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, requestLimit))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		platform.WriteError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if err := request.Normalize(); err != nil {
		platform.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	response, err := a.service.Quote(r.Context(), tenant, request)
	if err != nil {
		a.respondPartnerFailure(w, err)
		return
	}

	w.Header().Set("X-Tenant-Id", tenant)
	platform.WriteJSON(w, http.StatusOK, response)
}

// respondPartnerFailure translates the failure of the external dependency into a 502 — and says
// whose fault it was.
//
// A single partner being down brings down the entire request. That is the deliberately naive
// behavior: with no circuit breaker, no fallback and no cache, the availability of the platform is
// the product of the availability of the three partners.
func (a *API) respondPartnerFailure(w http.ResponseWriter, err error) {
	var failure *partner.Error
	if errors.As(err, &failure) {
		platform.WriteJSON(w, http.StatusBadGateway, platform.ErrorBody{
			Message: "partner insurer unavailable",
			Partner: failure.Partner,
		})
		return
	}
	platform.WriteError(w, http.StatusBadGateway, "failed to query the partner insurers")
}
