package quotation

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/GuilhermeOliveira591/mba_arquitetura_desafio_cotacao_seguros/internal/partner"
	"github.com/GuilhermeOliveira591/mba_arquitetura_desafio_cotacao_seguros/internal/platform"
)

var tresParceiras = []platform.Parceira{
	{Nome: "partner-slow", BaseURL: "http://slow"},
	{Nome: "partner-flaky", BaseURL: "http://flaky"},
	{Nome: "partner-degrading", BaseURL: "http://degrading"},
}

func pedidoValido() Pedido {
	p := Pedido{
		Condutor: Condutor{Documento: "12345678901", AnoNascimento: 1988},
		Veiculo:  Veiculo{Placa: "ABC1D23", Modelo: "Gol 1.0", Ano: 2020, ValorCentavos: 8500000},
	}
	if err := p.Normalizar(); err != nil {
		panic(err)
	}
	return p
}

// cotadorFalso substitui as parceiras nos testes e registra como foi chamado.
type cotadorFalso struct {
	demora      time.Duration
	premios     map[string]int64
	falhaEm     string
	chamadas    []string
	pedidos     []any
	simultaneas atomic.Int32
	pico        atomic.Int32
}

func (c *cotadorFalso) Cotar(_ context.Context, parceira platform.Parceira, pedido any) (partner.Cotacao, error) {
	if agora := c.simultaneas.Add(1); agora > c.pico.Load() {
		c.pico.Store(agora)
	}
	defer c.simultaneas.Add(-1)

	c.chamadas = append(c.chamadas, parceira.Nome)
	c.pedidos = append(c.pedidos, pedido)
	time.Sleep(c.demora)

	if parceira.Nome == c.falhaEm {
		return partner.Cotacao{}, &partner.Erro{Parceira: parceira.Nome, Status: 503, Motivo: "parceira respondeu 503"}
	}
	return partner.Cotacao{Parceira: parceira.Nome, PremioCentavos: c.premios[parceira.Nome], Moeda: "BRL"}, nil
}

func premiosPadrao() map[string]int64 {
	return map[string]int64{"partner-slow": 180000, "partner-flaky": 90000, "partner-degrading": 120000}
}

func TestCotarAgregaAsTresParceirasOrdenadasPeloPremio(t *testing.T) {
	cotador := &cotadorFalso{premios: premiosPadrao()}
	resposta, err := NovoServico(tresParceiras, cotador).Cotar(context.Background(), "corretora-a", pedidoValido())
	if err != nil {
		t.Fatalf("Cotar: %v", err)
	}

	if len(resposta.Cotacoes) != 3 {
		t.Fatalf("%d cotacoes, esperadas 3", len(resposta.Cotacoes))
	}
	esperado := []string{"partner-flaky", "partner-degrading", "partner-slow"} // do mais barato ao mais caro
	for i, nome := range esperado {
		if resposta.Cotacoes[i].Parceira != nome {
			t.Errorf("cotacao %d e de %q, esperada de %q", i, resposta.Cotacoes[i].Parceira, nome)
		}
	}
	if resposta.TenantID != "corretora-a" {
		t.Errorf("tenant_id %q, esperado corretora-a", resposta.TenantID)
	}
}

// TestCotarConsultaAsParceirasEmSerie trava a ingenuidade proposital do starter: se alguem
// paralelizar as chamadas, o cenario de degradacao do desafio deixa de aparecer e este teste quebra.
func TestCotarConsultaAsParceirasEmSerie(t *testing.T) {
	cotador := &cotadorFalso{demora: 40 * time.Millisecond, premios: premiosPadrao()}

	inicio := time.Now()
	if _, err := NovoServico(tresParceiras, cotador).Cotar(context.Background(), "corretora-a", pedidoValido()); err != nil {
		t.Fatalf("Cotar: %v", err)
	}
	decorrido := time.Since(inicio)

	if minimo := 3 * cotador.demora; decorrido < minimo {
		t.Fatalf("agregacao levou %s; em serie deveria levar ao menos %s", decorrido, minimo)
	}
	if pico := cotador.pico.Load(); pico != 1 {
		t.Fatalf("%d chamadas simultaneas no pico, esperada 1 (consulta em serie)", pico)
	}
}

// TestUmaParceiraForaDerrubaARequisicaoInteira e o outro vazio proposital: sem fallback, a resposta
// parcial nao existe.
func TestUmaParceiraForaDerrubaARequisicaoInteira(t *testing.T) {
	cotador := &cotadorFalso{premios: premiosPadrao(), falhaEm: "partner-flaky"}

	_, err := NovoServico(tresParceiras, cotador).Cotar(context.Background(), "corretora-a", pedidoValido())
	if err == nil {
		t.Fatal("uma parceira fora e a requisicao respondeu sucesso — ha fallback onde nao deveria haver")
	}

	var falha *partner.Erro
	if !errors.As(err, &falha) || falha.Parceira != "partner-flaky" {
		t.Fatalf("erro %v nao identifica a parceira que falhou", err)
	}
	// A parceira seguinte nem chega a ser consultada: a falha interrompe a serie.
	if ultima := cotador.chamadas[len(cotador.chamadas)-1]; ultima != "partner-flaky" {
		t.Errorf("ultima parceira consultada foi %q, esperada partner-flaky", ultima)
	}
}

// TestCorretoraVaiNoPedidoDaParceira garante que a mesma placa cotada por corretoras diferentes gera
// pedidos diferentes — o que torna a chave de cache por tenant uma exigencia real no PoC.
func TestCorretoraVaiNoPedidoDaParceira(t *testing.T) {
	cotador := &cotadorFalso{premios: premiosPadrao()}
	servico := NovoServico(tresParceiras[:1], cotador)

	if _, err := servico.Cotar(context.Background(), "corretora-a", pedidoValido()); err != nil {
		t.Fatalf("Cotar: %v", err)
	}
	if _, err := servico.Cotar(context.Background(), "corretora-b", pedidoValido()); err != nil {
		t.Fatalf("Cotar: %v", err)
	}

	paraA, paraB := serializar(t, cotador.pedidos[0]), serializar(t, cotador.pedidos[1])
	if paraA == paraB {
		t.Fatalf("corretoras diferentes geraram o mesmo pedido a parceira: %s", paraA)
	}

	var corpo map[string]any
	if err := json.Unmarshal([]byte(paraA), &corpo); err != nil {
		t.Fatalf("pedido a parceira nao e JSON: %v", err)
	}
	if corpo["broker"] != "corretora-a" {
		t.Errorf("broker %v, esperado corretora-a", corpo["broker"])
	}
	if _, tem := corpo["vehicle"]; !tem {
		t.Errorf("pedido a parceira nao carrega o veiculo: %s", paraA)
	}
}

func serializar(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return string(b)
}
