package main

import (
	"fmt"
	"hash/fnv"
	"sync/atomic"
	"time"
)

// Comportamento e o mau comportamento da parceira: quanto ela demora, quando ela falha e como ela
// piora sob carga.
//
// Determinismo e pre-requisito de correcao justa — dois alunos rodando o mesmo roteiro precisam ver
// a mesma sequencia de falhas. Por isso nao ha `math/rand` aqui: toda decisao e uma funcao pura da
// semente e do numero de sequencia da requisicao. A n-esima requisicao de um `partner-flaky` com a
// semente padrao falha (ou nao) igual na maquina de todo mundo, hoje e daqui a um ano.
//
// A contrapartida: sob concorrencia, quem recebe qual numero de sequencia depende da ordem de
// chegada. O que se repete e a *sequencia de respostas*, nao o pareamento requisicao-resposta.
type Comportamento struct {
	cfg   Config
	seq   atomic.Uint64
	emVoo atomic.Int64
}

func NovoComportamento(cfg Config) *Comportamento {
	return &Comportamento{cfg: cfg}
}

// Decisao e o veredito sobre uma requisicao, tomado inteiro no momento da chegada.
type Decisao struct {
	Sequencia uint64
	EmVoo     int64 // requisicoes em voo no instante da chegada, ja contando esta
	Latencia  time.Duration
	Falha     bool
}

// Admitir registra a chegada de uma requisicao e decide como ela sera atendida. Todo Admitir precisa
// ter um Concluir correspondente, senao a contagem de requisicoes em voo vaza e a parceira degrada
// para sempre.
func (c *Comportamento) Admitir() Decisao {
	seq := c.seq.Add(1)
	emVoo := c.emVoo.Add(1)
	return Decisao{
		Sequencia: seq,
		EmVoo:     emVoo,
		Latencia:  c.latencia(seq, emVoo),
		Falha:     c.falha(seq),
	}
}

// Concluir devolve a vaga da requisicao que terminou.
func (c *Comportamento) Concluir() {
	c.emVoo.Add(-1)
}

// EmVoo expoe a carga corrente da parceira.
func (c *Comportamento) EmVoo() int64 {
	return c.emVoo.Load()
}

// latencia soma as tres parcelas do atraso: a base constante do perfil, o jitter deterministico e a
// degradacao proporcional a carga.
func (c *Comportamento) latencia(seq uint64, emVoo int64) time.Duration {
	total := c.cfg.Latencia
	if c.cfg.Jitter > 0 {
		total += time.Duration(fracao(c.cfg.Semente, seq, fluxoJitter) * float64(c.cfg.Jitter))
	}
	return total + c.degradacao(emVoo)
}

// degradacao e o `partner-degrading`: enquanto a carga cabe no limiar, a parceira responde no tempo
// normal; acima dele, cada requisicao simultanea adiciona um passo de latencia, ate o teto. E o
// modelo de fila mais simples que ainda ensina a licao — a parceira nao quebra, ela afunda.
func (c *Comportamento) degradacao(emVoo int64) time.Duration {
	if c.cfg.DegradarApos <= 0 || c.cfg.DegradarPasso <= 0 || emVoo <= c.cfg.DegradarApos {
		return 0
	}
	extra := time.Duration(emVoo-c.cfg.DegradarApos) * c.cfg.DegradarPasso
	if c.cfg.DegradarTeto > 0 && extra > c.cfg.DegradarTeto {
		return c.cfg.DegradarTeto
	}
	return extra
}

// falha e o `partner-flaky`. A fracao derivada da semente distribui as falhas em rajadas, como uma
// parceira real — e nao alternadas de forma regular, que e o que uma divisao exata produziria. A
// rajada importa: circuit breaker que dispara por falhas consecutivas nunca abriria com um padrao
// perfeitamente intercalado.
func (c *Comportamento) falha(seq uint64) bool {
	switch {
	case c.cfg.TaxaFalha <= 0:
		return false
	case c.cfg.TaxaFalha >= 1:
		return true
	}
	return fracao(c.cfg.Semente, seq, fluxoFalha) < c.cfg.TaxaFalha
}

// Cotacao e a resposta de sucesso da parceira.
type Cotacao struct {
	Parceira          string `json:"partner"`
	CotacaoID         string `json:"quote_id"`
	PremioCentavos    int64  `json:"premium_cents"`
	Moeda             string `json:"currency"`
	CoberturaCentavos int64  `json:"coverage_cents"`
	ValidadeSegundos  int64  `json:"valid_for_seconds"`
}

// Cotar deriva a cotacao do conteudo do pedido, e nao do relogio nem de um contador: o mesmo pedido
// para a mesma parceira devolve sempre a mesma cotacao. E o que torna o efeito do cache visivel —
// hit e miss retornam valores iguais, entao a diferenca que o aluno mede e de latencia e de custo,
// nao de conteudo.
func (c *Comportamento) Cotar(pedido []byte) Cotacao {
	h := fnv.New64a()
	_, _ = h.Write([]byte(c.cfg.Nome))
	_, _ = h.Write(pedido)
	soma := h.Sum64()

	return Cotacao{
		Parceira:  c.cfg.Nome,
		CotacaoID: fmt.Sprintf("%s-%016x", c.cfg.Nome, soma),
		// Premio entre R$ 500,00 e R$ 2.000,00.
		PremioCentavos: 50000 + int64(soma%150001),
		Moeda:          "BRL",
		// Cobertura entre R$ 30.000,00 e R$ 100.000,00, de dez em dez mil.
		CoberturaCentavos: 3000000 + int64((soma>>32)%8)*1000000,
		ValidadeSegundos:  int64(c.cfg.ValidadeCotacao.Seconds()),
	}
}

// Fluxos independentes de decisao: sem eles, latencia e falha andariam juntas (toda requisicao lenta
// seria tambem a que falha), o que seria um padrao facil de decorar e nao de diagnosticar.
const (
	fluxoFalha  uint64 = 1
	fluxoJitter uint64 = 2
)

// fracao devolve um valor em [0,1) a partir da semente, do numero de sequencia e do fluxo. Mesmas
// entradas, mesmo valor — sempre, em qualquer maquina.
func fracao(semente, seq, fluxo uint64) float64 {
	v := misturar(misturar(semente+seq*0x9e3779b97f4a7c15) ^ fluxo)
	return float64(v>>11) / float64(uint64(1)<<53)
}

// misturar e o finalizador do splitmix64: espalha bits de forma barata e reprodutivel.
func misturar(x uint64) uint64 {
	x += 0x9e3779b97f4a7c15
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	return x ^ (x >> 31)
}
