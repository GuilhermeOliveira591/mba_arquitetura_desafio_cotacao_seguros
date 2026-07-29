package quotation

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/GuilhermeOliveira591/mba_arquitetura_desafio_cotacao_seguros/internal/platform"
)

const corpoValido = `{
  "driver": {"document": "12345678901", "birth_year": 1988},
  "vehicle": {"plate": "abc1d23", "model": "Gol 1.0", "year": 2020, "value_cents": 8500000},
  "coverage": "comprehensive"
}`

func apiDeTeste(cotador Cotador) http.Handler {
	return NovaAPI(NovoServico(tresParceiras, cotador), []string{"corretora-a", "corretora-b"}).Rotas()
}

func postQuotes(h http.Handler, tenant, corpo string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/quotes", strings.NewReader(corpo))
	if tenant != "" {
		req.Header.Set(CabecalhoTenant, tenant)
	}
	resposta := httptest.NewRecorder()
	h.ServeHTTP(resposta, req)
	return resposta
}

func TestQuotesRetornaCotacoesAgregadas(t *testing.T) {
	resposta := postQuotes(apiDeTeste(&cotadorFalso{premios: premiosPadrao()}), "corretora-a", corpoValido)
	if resposta.Code != http.StatusOK {
		t.Fatalf("status %d, esperado 200: %s", resposta.Code, resposta.Body)
	}

	var corpo Resposta
	if err := json.Unmarshal(resposta.Body.Bytes(), &corpo); err != nil {
		t.Fatalf("resposta ilegivel: %v", err)
	}
	if len(corpo.Cotacoes) != 3 {
		t.Fatalf("%d cotacoes, esperadas 3", len(corpo.Cotacoes))
	}
	if corpo.TenantID != "corretora-a" {
		t.Errorf("tenant_id %q, esperado corretora-a", corpo.TenantID)
	}
}

// TestQuotesRejeitaRequisicaoSemTenant e o criterio de aceite da issue: sem corretora, sem cotacao.
func TestQuotesRejeitaRequisicaoSemTenant(t *testing.T) {
	cotador := &cotadorFalso{premios: premiosPadrao()}
	resposta := postQuotes(apiDeTeste(cotador), "", corpoValido)

	if resposta.Code != http.StatusBadRequest {
		t.Fatalf("status %d, esperado 400", resposta.Code)
	}
	if len(cotador.chamadas) != 0 {
		t.Fatalf("as parceiras foram consultadas (%v) mesmo sem tenant", cotador.chamadas)
	}
}

func TestQuotesRejeitaCorretoraDesconhecida(t *testing.T) {
	cotador := &cotadorFalso{premios: premiosPadrao()}
	resposta := postQuotes(apiDeTeste(cotador), "corretora-pirata", corpoValido)

	if resposta.Code != http.StatusForbidden {
		t.Fatalf("status %d, esperado 403", resposta.Code)
	}
	if len(cotador.chamadas) != 0 {
		t.Fatalf("as parceiras foram consultadas (%v) para uma corretora nao habilitada", cotador.chamadas)
	}
}

func TestQuotesRejeitaCorpoInvalido(t *testing.T) {
	casos := map[string]string{
		"json quebrado":      `{"driver":`,
		"sem documento":      `{"driver":{"birth_year":1988},"vehicle":{"plate":"ABC1D23","year":2020,"value_cents":100}}`,
		"sem placa":          `{"driver":{"document":"1","birth_year":1988},"vehicle":{"year":2020,"value_cents":100}}`,
		"valor zerado":       `{"driver":{"document":"1","birth_year":1988},"vehicle":{"plate":"A","year":2020,"value_cents":0}}`,
		"cobertura invalida": `{"driver":{"document":"1","birth_year":1988},"vehicle":{"plate":"A","year":2020,"value_cents":1},"coverage":"vip"}`,
		"campo desconhecido": `{"driver":{"document":"1","birth_year":1988},"vehicle":{"plate":"A","year":2020,"value_cents":1},"desconto":true}`,
	}

	for nome, corpo := range casos {
		t.Run(nome, func(t *testing.T) {
			resposta := postQuotes(apiDeTeste(&cotadorFalso{premios: premiosPadrao()}), "corretora-a", corpo)
			if resposta.Code != http.StatusBadRequest {
				t.Fatalf("status %d, esperado 400: %s", resposta.Code, resposta.Body)
			}
		})
	}
}

// TestQuotesResponde502ComANomeDaParceiraQueCaiu documenta o comportamento ingenuo: nao ha resposta
// parcial, e a API diz de quem foi a culpa.
func TestQuotesResponde502ComANomeDaParceiraQueCaiu(t *testing.T) {
	cotador := &cotadorFalso{premios: premiosPadrao(), falhaEm: "partner-flaky"}
	resposta := postQuotes(apiDeTeste(cotador), "corretora-a", corpoValido)

	if resposta.Code != http.StatusBadGateway {
		t.Fatalf("status %d, esperado 502", resposta.Code)
	}

	var erro platform.Erro
	if err := json.Unmarshal(resposta.Body.Bytes(), &erro); err != nil {
		t.Fatalf("erro ilegivel: %v", err)
	}
	if erro.Parceira != "partner-flaky" {
		t.Errorf("parceira culpada %q, esperada partner-flaky", erro.Parceira)
	}
}

func TestHealthz(t *testing.T) {
	resposta := httptest.NewRecorder()
	apiDeTeste(&cotadorFalso{premios: premiosPadrao()}).
		ServeHTTP(resposta, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if resposta.Code != http.StatusOK {
		t.Fatalf("status %d, esperado 200", resposta.Code)
	}
}
