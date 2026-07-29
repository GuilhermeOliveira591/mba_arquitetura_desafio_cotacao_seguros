package quotation

import (
	"context"
	"sort"
	"time"

	"github.com/GuilhermeOliveira591/mba_arquitetura_desafio_cotacao_seguros/internal/partner"
	"github.com/GuilhermeOliveira591/mba_arquitetura_desafio_cotacao_seguros/internal/platform"
)

// Cotador e o que o servico precisa saber sobre uma parceira.
type Cotador interface {
	Cotar(ctx context.Context, parceira platform.Parceira, pedido any) (partner.Cotacao, error)
}

// Servico agrega as cotacoes das parceiras.
type Servico struct {
	parceiras []platform.Parceira
	cotador   Cotador
}

func NovoServico(parceiras []platform.Parceira, cotador Cotador) *Servico {
	return &Servico{parceiras: parceiras, cotador: cotador}
}

// Cotar consulta as parceiras EM SERIE e devolve as cotacoes ordenadas da mais barata para a mais
// cara.
//
// Duas ingenuidades aqui sao propositais e sao o coracao do desafio:
//
//  1. Serie, nao paralelo. O tempo da resposta e a SOMA do tempo das tres parceiras. Com a
//     partner-slow em 1,5s, nenhuma cotacao sai abaixo disso — e sob carga a partner-degrading
//     empilha em cima.
//  2. Tudo ou nada. Uma parceira que falha derruba a requisicao inteira, mesmo que as outras duas
//     tenham respondido. Nao ha fallback, nao ha resposta parcial, nao ha cache para servir a
//     cotacao anterior.
//
// E exatamente a patologia que o aluno vai medir antes e depois. Nao "conserte" isto no starter: o
// vazio e o enunciado.
func (s *Servico) Cotar(ctx context.Context, tenant string, pedido Pedido) (Resposta, error) {
	inicio := time.Now()
	paraParceira := pedidoParceira{Corretora: tenant, Pedido: pedido}

	cotacoes := make([]partner.Cotacao, 0, len(s.parceiras))
	for _, parceira := range s.parceiras {
		cotacao, err := s.cotador.Cotar(ctx, parceira, paraParceira)
		if err != nil {
			return Resposta{}, err
		}
		cotacoes = append(cotacoes, cotacao)
	}

	sort.Slice(cotacoes, func(i, j int) bool {
		return cotacoes[i].PremioCentavos < cotacoes[j].PremioCentavos
	})

	return Resposta{
		TenantID: tenant,
		Cotacoes: cotacoes,
		TempoMs:  time.Since(inicio).Milliseconds(),
	}, nil
}
