package quotation

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/GuilhermeOliveira591/mba_arquitetura_desafio_cotacao_seguros/internal/partner"
	"github.com/GuilhermeOliveira591/mba_arquitetura_desafio_cotacao_seguros/internal/platform"
)

// CabecalhoTenant identifica a corretora que esta pedindo a cotacao.
const CabecalhoTenant = "X-Tenant-Id"

// limitePedido corta corpos absurdos antes de eles virarem memoria.
const limitePedido = 1 << 20 // 1 MiB

// API expoe o contrato HTTP da quotation-api.
type API struct {
	servico *Servico
	tenants map[string]bool
}

func NovaAPI(servico *Servico, tenants []string) *API {
	conhecidos := make(map[string]bool, len(tenants))
	for _, t := range tenants {
		conhecidos[t] = true
	}
	return &API{servico: servico, tenants: conhecidos}
}

// Rotas monta o roteador da API.
//
//	POST /quotes   cotacao agregada das parceiras (exige X-Tenant-Id)
//	GET  /healthz  saude do processo
func (a *API) Rotas() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /quotes", a.cotar)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		platform.EscreverJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	return mux
}

func (a *API) cotar(w http.ResponseWriter, r *http.Request) {
	tenant := r.Header.Get(CabecalhoTenant)
	if tenant == "" {
		platform.EscreverErro(w, http.StatusBadRequest, CabecalhoTenant+" e obrigatorio")
		return
	}
	// Corretora desconhecida nao e "nao autenticado", e "nao e sua". O isolamento entre corretoras e
	// requisito do cenario (SUSEP e LGPD), nao detalhe de implementacao.
	if !a.tenants[tenant] {
		platform.EscreverErro(w, http.StatusForbidden, "corretora nao habilitada nesta plataforma")
		return
	}

	var pedido Pedido
	decodificador := json.NewDecoder(http.MaxBytesReader(w, r.Body, limitePedido))
	decodificador.DisallowUnknownFields()
	if err := decodificador.Decode(&pedido); err != nil {
		platform.EscreverErro(w, http.StatusBadRequest, "corpo invalido: "+err.Error())
		return
	}
	if err := pedido.Normalizar(); err != nil {
		platform.EscreverErro(w, http.StatusBadRequest, err.Error())
		return
	}

	resposta, err := a.servico.Cotar(r.Context(), tenant, pedido)
	if err != nil {
		a.responderFalhaDeParceira(w, err)
		return
	}

	w.Header().Set("X-Tenant-Id", tenant)
	platform.EscreverJSON(w, http.StatusOK, resposta)
}

// responderFalhaDeParceira traduz a falha da dependencia externa em 502 — e diz de quem foi.
//
// Uma unica parceira fora derruba a requisicao inteira. E o comportamento ingenuo de proposito: sem
// circuit breaker, sem fallback e sem cache, a disponibilidade da plataforma e o produto da
// disponibilidade das tres parceiras.
func (a *API) responderFalhaDeParceira(w http.ResponseWriter, err error) {
	var falha *partner.Erro
	if errors.As(err, &falha) {
		platform.EscreverJSON(w, http.StatusBadGateway, platform.Erro{
			Mensagem: "seguradora parceira indisponivel",
			Parceira: falha.Parceira,
		})
		return
	}
	platform.EscreverErro(w, http.StatusBadGateway, "falha ao consultar as seguradoras parceiras")
}
