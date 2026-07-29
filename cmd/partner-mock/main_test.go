package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func pedirCotacao(t *testing.T, h http.Handler, corpo string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/quotes", strings.NewReader(corpo))
	resposta := httptest.NewRecorder()
	h.ServeHTTP(resposta, req)
	return resposta
}

func TestCotarRespondeCotacaoQuandoAParceiraEstaSa(t *testing.T) {
	cfg := perfilFlaky()
	cfg.Latencia, cfg.Jitter, cfg.TaxaFalha = 0, 0, 0
	h := rotas(cfg, NovoComportamento(cfg))

	resposta := pedirCotacao(t, h, `{"tenant_id":"corretora-a"}`)
	if resposta.Code != http.StatusOK {
		t.Fatalf("status %d, esperado 200", resposta.Code)
	}

	var cotacao Cotacao
	if err := json.Unmarshal(resposta.Body.Bytes(), &cotacao); err != nil {
		t.Fatalf("resposta nao e uma cotacao: %v", err)
	}
	if cotacao.Parceira != cfg.Nome || cotacao.CotacaoID == "" || cotacao.PremioCentavos == 0 {
		t.Fatalf("cotacao incompleta: %+v", cotacao)
	}

	if got := resposta.Header().Get("X-Partner-Seq"); got != "1" {
		t.Errorf("X-Partner-Seq %q, esperado 1", got)
	}
	if got := resposta.Header().Get("X-Partner-Name"); got != cfg.Nome {
		t.Errorf("X-Partner-Name %q, esperado %q", got, cfg.Nome)
	}
	if got := resposta.Header().Get("X-Partner-Inflight"); got != "1" {
		t.Errorf("X-Partner-Inflight %q, esperado 1", got)
	}
}

func TestCotarFalhaComOStatusConfigurado(t *testing.T) {
	cfg := perfilFlaky()
	cfg.Latencia, cfg.Jitter, cfg.TaxaFalha, cfg.StatusFalha = 0, 0, 1, 502
	h := rotas(cfg, NovoComportamento(cfg))

	resposta := pedirCotacao(t, h, `{}`)
	if resposta.Code != 502 {
		t.Fatalf("status %d, esperado 502", resposta.Code)
	}

	var erro erroJSON
	if err := json.Unmarshal(resposta.Body.Bytes(), &erro); err != nil {
		t.Fatalf("resposta de erro nao e JSON: %v", err)
	}
	if erro.Parceira != cfg.Nome || erro.Erro == "" {
		t.Fatalf("erro incompleto: %+v", erro)
	}
}

func TestCotarAplicaALatenciaDoPerfil(t *testing.T) {
	cfg := perfilFlaky()
	cfg.Latencia, cfg.Jitter, cfg.TaxaFalha = 80*time.Millisecond, 0, 0
	h := rotas(cfg, NovoComportamento(cfg))

	inicio := time.Now()
	resposta := pedirCotacao(t, h, `{}`)
	decorrido := time.Since(inicio)

	if decorrido < cfg.Latencia {
		t.Fatalf("resposta levou %s, esperado ao menos %s", decorrido, cfg.Latencia)
	}
	if got := resposta.Header().Get("X-Partner-Latency-Ms"); got != "80" {
		t.Errorf("X-Partner-Latency-Ms %q, esperado 80", got)
	}
}

// TestCotarDesisteQuandoOClienteDesiste protege o comportamento que o aluno vai exercitar com timeout
// e circuit breaker: a parceira lenta nao pode segurar goroutine de requisicao ja abandonada.
func TestCotarDesisteQuandoOClienteDesiste(t *testing.T) {
	cfg := perfilFlaky()
	cfg.Latencia, cfg.Jitter, cfg.TaxaFalha = 5*time.Second, 0, 0
	comportamento := NovoComportamento(cfg)
	h := rotas(cfg, comportamento)

	ctx, cancelar := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancelar()
	req := httptest.NewRequest(http.MethodPost, "/quotes", strings.NewReader(`{}`)).WithContext(ctx)

	inicio := time.Now()
	h.ServeHTTP(httptest.NewRecorder(), req)
	if decorrido := time.Since(inicio); decorrido > time.Second {
		t.Fatalf("handler levou %s depois do cliente desistir", decorrido)
	}
	if emVoo := comportamento.EmVoo(); emVoo != 0 {
		t.Fatalf("%d requisicoes em voo depois do cancelamento, esperado 0", emVoo)
	}
}

// TestHealthzIgnoraOPerfil e o que mantem o `docker compose up` viavel: uma parceira com 5s de
// latencia e 100% de falha ainda precisa subir saudavel.
func TestHealthzIgnoraOPerfil(t *testing.T) {
	cfg := perfilFlaky()
	cfg.Latencia, cfg.TaxaFalha = 5*time.Second, 1
	h := rotas(cfg, NovoComportamento(cfg))

	inicio := time.Now()
	resposta := httptest.NewRecorder()
	h.ServeHTTP(resposta, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if resposta.Code != http.StatusOK {
		t.Fatalf("status %d, esperado 200", resposta.Code)
	}
	if decorrido := time.Since(inicio); decorrido > time.Second {
		t.Fatalf("healthz levou %s — o perfil vazou para o healthcheck", decorrido)
	}
}

func TestConfigExpoeOPerfilEfetivo(t *testing.T) {
	cfg := perfilFlaky()
	h := rotas(cfg, NovoComportamento(cfg))

	resposta := httptest.NewRecorder()
	h.ServeHTTP(resposta, httptest.NewRequest(http.MethodGet, "/config", nil))
	if resposta.Code != http.StatusOK {
		t.Fatalf("status %d, esperado 200", resposta.Code)
	}

	var corpo map[string]any
	if err := json.Unmarshal(resposta.Body.Bytes(), &corpo); err != nil {
		t.Fatalf("resposta nao e JSON: %v", err)
	}
	if corpo["partner"] != cfg.Nome {
		t.Errorf("partner %v, esperado %q", corpo["partner"], cfg.Nome)
	}
	if corpo["failure_rate"] != cfg.TaxaFalha {
		t.Errorf("failure_rate %v, esperado %v", corpo["failure_rate"], cfg.TaxaFalha)
	}
	if corpo["latency_ms"] != float64(cfg.Latencia.Milliseconds()) {
		t.Errorf("latency_ms %v, esperado %d", corpo["latency_ms"], cfg.Latencia.Milliseconds())
	}
}
