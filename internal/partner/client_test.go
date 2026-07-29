package partner

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/GuilhermeOliveira591/mba_arquitetura_desafio_cotacao_seguros/internal/platform"
)

func parceiraDeTeste(h http.Handler) (platform.Parceira, func()) {
	servidor := httptest.NewServer(h)
	return platform.Parceira{Nome: "partner-flaky", BaseURL: servidor.URL}, servidor.Close
}

func TestCotarLeARespostaDaParceira(t *testing.T) {
	var caminho, tipo string
	var recebido map[string]any

	parceira, fechar := parceiraDeTeste(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		caminho, tipo = r.URL.Path, r.Header.Get("Content-Type")
		corpo, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(corpo, &recebido)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"partner":"ignorado","quote_id":"q-1","premium_cents":123456,` +
			`"currency":"BRL","coverage_cents":5000000,"valid_for_seconds":300}`))
	}))
	defer fechar()

	cotacao, err := NovoCliente().Cotar(context.Background(), parceira, map[string]string{"broker": "corretora-a"})
	if err != nil {
		t.Fatalf("Cotar: %v", err)
	}

	if caminho != "/quotes" {
		t.Errorf("caminho %q, esperado /quotes", caminho)
	}
	if tipo != "application/json" {
		t.Errorf("Content-Type %q, esperado application/json", tipo)
	}
	if recebido["broker"] != "corretora-a" {
		t.Errorf("parceira recebeu broker %v, esperado corretora-a", recebido["broker"])
	}
	if cotacao.PremioCentavos != 123456 || cotacao.CotacaoID != "q-1" {
		t.Errorf("cotacao lida errado: %+v", cotacao)
	}
	// O nome vem da configuracao, nao do que a parceira disser de si mesma.
	if cotacao.Parceira != "partner-flaky" {
		t.Errorf("parceira %q, esperada partner-flaky", cotacao.Parceira)
	}
}

func TestCotarIdentificaAParceiraQueFalhou(t *testing.T) {
	parceira, fechar := parceiraDeTeste(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"parceira indisponivel"}`, http.StatusServiceUnavailable)
	}))
	defer fechar()

	_, err := NovoCliente().Cotar(context.Background(), parceira, map[string]string{})
	if err == nil {
		t.Fatal("503 da parceira foi tratado como sucesso")
	}

	var falha *Erro
	if !errors.As(err, &falha) {
		t.Fatalf("erro %v nao e *partner.Erro", err)
	}
	if falha.Parceira != "partner-flaky" || falha.Status != http.StatusServiceUnavailable {
		t.Fatalf("erro nao identifica a falha: %+v", falha)
	}
}

func TestCotarFalhaComRespostaIlegivel(t *testing.T) {
	parceira, fechar := parceiraDeTeste(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("nao sou json"))
	}))
	defer fechar()

	if _, err := NovoCliente().Cotar(context.Background(), parceira, map[string]string{}); err == nil {
		t.Fatal("resposta ilegivel foi aceita")
	}
}

// TestCotarRespeitaOCancelamento mostra que a unica protecao existente hoje e a que vem do chamador:
// nao ha timeout proprio no cliente — e isso e o vazio proposital do starter.
func TestCotarRespeitaOCancelamento(t *testing.T) {
	parceira, fechar := parceiraDeTeste(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer fechar()

	ctx, cancelar := context.WithCancel(context.Background())
	cancelar()

	if _, err := NovoCliente().Cotar(ctx, parceira, map[string]string{}); err == nil {
		t.Fatal("contexto cancelado nao interrompeu a chamada")
	}
}
