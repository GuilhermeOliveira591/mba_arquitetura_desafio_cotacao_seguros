// Package platform reune a infraestrutura compartilhada da quotation-api: configuracao, servidor
// HTTP e escrita de JSON. Nada de regra de negocio mora aqui.
package platform

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

// Config e a configuracao da quotation-api.
type Config struct {
	Porta     string     // PORT
	Parceiras []Parceira // PARTNER_ENDPOINTS
	Tenants   []string   // TENANTS
}

// Parceira e uma seguradora parceira alcancavel por HTTP.
type Parceira struct {
	Nome    string
	BaseURL string
}

// enderecosPadrao aponta para as portas que o docker-compose.yml publica no host, para `make run`
// funcionar contra o ambiente subido com `make up` sem precisar exportar nada.
const enderecosPadrao = "partner-slow=http://localhost:9001," +
	"partner-flaky=http://localhost:9002," +
	"partner-degrading=http://localhost:9003"

// tenantsPadrao sao as corretoras que o starter ja conhece. Multi-tenant aqui nao e enfeite: e o
// que torna a isolacao do cache uma decisao de verdade quando o aluno chegar no PoC.
const tenantsPadrao = "corretora-a,corretora-b"

func CarregarConfig(env func(string) string) (Config, error) {
	cfg := Config{Porta: texto(env, "PORT", "8080")}

	var err error
	if cfg.Parceiras, err = parsearParceiras(texto(env, "PARTNER_ENDPOINTS", enderecosPadrao)); err != nil {
		return Config{}, err
	}
	if cfg.Tenants, err = parsearTenants(texto(env, "TENANTS", tenantsPadrao)); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// parsearParceiras le a lista `nome=url,nome=url`. Ter as tres parceiras em uma variavel so mantem a
// topologia do ambiente visivel em um lugar unico do docker-compose.yml.
func parsearParceiras(bruto string) ([]Parceira, error) {
	var parceiras []Parceira
	vistas := map[string]bool{}

	for _, item := range strings.Split(bruto, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}

		nome, endereco, achou := strings.Cut(item, "=")
		nome, endereco = strings.TrimSpace(nome), strings.TrimSpace(endereco)
		if !achou || nome == "" || endereco == "" {
			return nil, fmt.Errorf("PARTNER_ENDPOINTS: %q nao esta no formato nome=url", item)
		}

		if u, err := url.Parse(endereco); err != nil || u.Scheme == "" || u.Host == "" {
			return nil, fmt.Errorf("PARTNER_ENDPOINTS: %q nao e uma URL absoluta", endereco)
		}
		if vistas[nome] {
			return nil, fmt.Errorf("PARTNER_ENDPOINTS: parceira %q repetida", nome)
		}

		vistas[nome] = true
		parceiras = append(parceiras, Parceira{Nome: nome, BaseURL: strings.TrimRight(endereco, "/")})
	}

	if len(parceiras) == 0 {
		return nil, fmt.Errorf("PARTNER_ENDPOINTS: ao menos uma parceira e obrigatoria")
	}
	return parceiras, nil
}

func parsearTenants(bruto string) ([]string, error) {
	var tenants []string
	for _, t := range strings.Split(bruto, ",") {
		if t = strings.TrimSpace(t); t != "" {
			tenants = append(tenants, t)
		}
	}
	if len(tenants) == 0 {
		return nil, fmt.Errorf("TENANTS: ao menos uma corretora e obrigatoria")
	}
	return tenants, nil
}

// Ambiente e a leitura padrao de variaveis de ambiente.
func Ambiente() func(string) string { return os.Getenv }

func texto(env func(string) string, chave, padrao string) string {
	if v := env(chave); v != "" {
		return v
	}
	return padrao
}
