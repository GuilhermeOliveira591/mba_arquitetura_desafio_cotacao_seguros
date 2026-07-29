// Package partner e o cliente das seguradoras parceiras — a fronteira entre a quotation-api e a
// dependencia externa instavel do desafio.
//
// E aqui que o circuit breaker vai nascer. Hoje, de proposito, nao ha nenhuma protecao: sem timeout,
// sem retry, sem breaker, sem fallback. Uma chamada sai, e o que voltar (ou nao voltar) e o que a API
// entrega. Esse vazio e o exercicio, nao um esquecimento — ver a nota em Cliente.
package partner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/GuilhermeOliveira591/mba_arquitetura_desafio_cotacao_seguros/internal/platform"
)

// Cotacao e a resposta de uma seguradora parceira.
type Cotacao struct {
	Parceira          string `json:"partner"`
	CotacaoID         string `json:"quote_id"`
	PremioCentavos    int64  `json:"premium_cents"`
	Moeda             string `json:"currency"`
	CoberturaCentavos int64  `json:"coverage_cents"`
	ValidadeSegundos  int64  `json:"valid_for_seconds"`
}

// Cliente fala HTTP com as parceiras.
//
// O http.Client e deliberadamente cru: `Timeout` zero significa esperar para sempre. Uma parceira que
// degrada ate 6s segura a goroutine da requisicao esse tempo inteiro, e nada aqui impede a proxima
// chamada de fazer o mesmo. Colocar um timeout e a primeira coisa que o aluno vai querer fazer — e e
// justamente o que o starter nao entrega pronto.
type Cliente struct {
	http *http.Client
}

func NovoCliente() *Cliente {
	return &Cliente{http: &http.Client{}}
}

// limiteResposta corta respostas absurdas de uma parceira mal comportada.
const limiteResposta = 1 << 20 // 1 MiB

// Cotar pede uma cotacao a uma parceira. O pedido vai serializado pelo proprio cliente, e nao
// repassado byte a byte do cliente original: o corpo canonico faz a mesma cotacao logica produzir
// sempre a mesma requisicao — o que a parceira responde de forma estavel e o que, mais tarde, torna
// uma chave de cache possivel.
func (c *Cliente) Cotar(ctx context.Context, parceira platform.Parceira, pedido any) (Cotacao, error) {
	corpo, err := json.Marshal(pedido)
	if err != nil {
		return Cotacao{}, &Erro{Parceira: parceira.Nome, Motivo: fmt.Sprintf("pedido invalido: %v", err)}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, parceira.BaseURL+"/quotes", bytes.NewReader(corpo))
	if err != nil {
		return Cotacao{}, &Erro{Parceira: parceira.Nome, Motivo: err.Error()}
	}
	req.Header.Set("Content-Type", "application/json")

	resposta, err := c.http.Do(req)
	if err != nil {
		return Cotacao{}, &Erro{Parceira: parceira.Nome, Motivo: err.Error()}
	}
	defer func() { _ = resposta.Body.Close() }()

	if resposta.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resposta.Body, limiteResposta))
		return Cotacao{}, &Erro{
			Parceira: parceira.Nome,
			Status:   resposta.StatusCode,
			Motivo:   fmt.Sprintf("parceira respondeu %d", resposta.StatusCode),
		}
	}

	var cotacao Cotacao
	if err := json.NewDecoder(io.LimitReader(resposta.Body, limiteResposta)).Decode(&cotacao); err != nil {
		return Cotacao{}, &Erro{Parceira: parceira.Nome, Motivo: fmt.Sprintf("resposta ilegivel: %v", err)}
	}

	// A parceira pode omitir o proprio nome; quem manda e a configuracao.
	cotacao.Parceira = parceira.Nome
	return cotacao, nil
}

// Erro identifica qual parceira falhou e por que. Sem isso a API so saberia dizer "deu erro" — e o
// aluno precisa saber de quem foi a culpa para decidir onde o breaker entra.
type Erro struct {
	Parceira string
	Status   int
	Motivo   string
}

func (e *Erro) Error() string {
	return fmt.Sprintf("parceira %s: %s", e.Parceira, e.Motivo)
}
