// Package quotation e a regra de cotacao e o contrato HTTP da quotation-api.
package quotation

import (
	"fmt"
	"strings"

	"github.com/GuilhermeOliveira591/mba_arquitetura_desafio_cotacao_seguros/internal/partner"
)

// Pedido e o corpo de POST /quotes: o risco a ser cotado. A corretora NAO aparece aqui — ela vem no
// cabecalho X-Tenant-Id, porque tenant e contexto da chamada, nao dado do seguro.
type Pedido struct {
	Condutor  Condutor `json:"driver"`
	Veiculo   Veiculo  `json:"vehicle"`
	Cobertura string   `json:"coverage"`
}

type Condutor struct {
	Documento     string `json:"document"`
	AnoNascimento int    `json:"birth_year"`
}

type Veiculo struct {
	Placa         string `json:"plate"`
	Modelo        string `json:"model"`
	Ano           int    `json:"year"`
	ValorCentavos int64  `json:"value_cents"`
}

// coberturas aceitas. Duas bastam: o starter e esqueleto funcional, nao produto.
var coberturas = map[string]bool{"comprehensive": true, "third_party": true}

const coberturaPadrao = "comprehensive"

// Normalizar preenche o que tem padrao e valida o resto. A validacao e curta de proposito — o
// desafio nao e sobre regra de negocio de seguro, e sobre o que acontece quando a parceira falha.
func (p *Pedido) Normalizar() error {
	p.Cobertura = strings.TrimSpace(p.Cobertura)
	if p.Cobertura == "" {
		p.Cobertura = coberturaPadrao
	}

	p.Condutor.Documento = strings.TrimSpace(p.Condutor.Documento)
	p.Veiculo.Placa = strings.ToUpper(strings.TrimSpace(p.Veiculo.Placa))
	p.Veiculo.Modelo = strings.TrimSpace(p.Veiculo.Modelo)

	switch {
	case p.Condutor.Documento == "":
		return fmt.Errorf("driver.document e obrigatorio")
	case p.Condutor.AnoNascimento < 1900:
		return fmt.Errorf("driver.birth_year e obrigatorio e precisa ser um ano valido")
	case p.Veiculo.Placa == "":
		return fmt.Errorf("vehicle.plate e obrigatorio")
	case p.Veiculo.Ano < 1900:
		return fmt.Errorf("vehicle.year e obrigatorio e precisa ser um ano valido")
	case p.Veiculo.ValorCentavos <= 0:
		return fmt.Errorf("vehicle.value_cents precisa ser maior que zero")
	case !coberturas[p.Cobertura]:
		return fmt.Errorf("coverage precisa ser comprehensive ou third_party")
	}
	return nil
}

// pedidoParceira e o que a parceira recebe. A corretora entra no corpo porque, no cenario do desafio,
// cada corretora tem sua propria condicao comercial: a mesma placa cotada por duas corretoras vale
// premios diferentes. E o que faz o isolamento por tenant ser exigencia, e nao zelo — inclusive na
// chave de cache que o aluno vai construir.
type pedidoParceira struct {
	Corretora string `json:"broker"`
	Pedido
}

// Resposta e o corpo de sucesso de POST /quotes.
type Resposta struct {
	TenantID string            `json:"tenant_id"`
	Cotacoes []partner.Cotacao `json:"quotes"`
	TempoMs  int64             `json:"elapsed_ms"`
}
