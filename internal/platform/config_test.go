package platform

import (
	"reflect"
	"testing"
)

func env(pares map[string]string) func(string) string {
	return func(chave string) string { return pares[chave] }
}

func TestConfigPadraoApontaParaOAmbienteDoCompose(t *testing.T) {
	cfg, err := CarregarConfig(env(nil))
	if err != nil {
		t.Fatalf("CarregarConfig: %v", err)
	}

	if cfg.Porta != "8080" {
		t.Errorf("porta %q, esperada 8080", cfg.Porta)
	}
	esperadas := []Parceira{
		{Nome: "partner-slow", BaseURL: "http://localhost:9001"},
		{Nome: "partner-flaky", BaseURL: "http://localhost:9002"},
		{Nome: "partner-degrading", BaseURL: "http://localhost:9003"},
	}
	if !reflect.DeepEqual(cfg.Parceiras, esperadas) {
		t.Errorf("parceiras %+v, esperadas %+v", cfg.Parceiras, esperadas)
	}
	if !reflect.DeepEqual(cfg.Tenants, []string{"corretora-a", "corretora-b"}) {
		t.Errorf("corretoras %v, esperadas corretora-a e corretora-b", cfg.Tenants)
	}
}

func TestConfigLeParceirasETenantsDoAmbiente(t *testing.T) {
	cfg, err := CarregarConfig(env(map[string]string{
		"PORT":              "9090",
		"PARTNER_ENDPOINTS": " a=http://a:8080/ , b=http://b:8080 ",
		"TENANTS":           " corretora-x , corretora-y ",
	}))
	if err != nil {
		t.Fatalf("CarregarConfig: %v", err)
	}

	esperadas := []Parceira{{Nome: "a", BaseURL: "http://a:8080"}, {Nome: "b", BaseURL: "http://b:8080"}}
	if !reflect.DeepEqual(cfg.Parceiras, esperadas) {
		t.Errorf("parceiras %+v, esperadas %+v", cfg.Parceiras, esperadas)
	}
	if !reflect.DeepEqual(cfg.Tenants, []string{"corretora-x", "corretora-y"}) {
		t.Errorf("corretoras %v inesperadas", cfg.Tenants)
	}
	if cfg.Porta != "9090" {
		t.Errorf("porta %q, esperada 9090", cfg.Porta)
	}
}

func TestConfigInvalidaFalha(t *testing.T) {
	casos := map[string]map[string]string{
		"parceira sem url":         {"PARTNER_ENDPOINTS": "partner-slow"},
		"url relativa":             {"PARTNER_ENDPOINTS": "partner-slow=/quotes"},
		"url sem host":             {"PARTNER_ENDPOINTS": "partner-slow=http://"},
		"parceira sem nome":        {"PARTNER_ENDPOINTS": "=http://a:8080"},
		"parceira repetida":        {"PARTNER_ENDPOINTS": "a=http://a:8080,a=http://b:8080"},
		"lista de parceiras vazia": {"PARTNER_ENDPOINTS": " , "},
		"lista de tenants vazia":   {"TENANTS": " , "},
	}

	for nome, vars := range casos {
		t.Run(nome, func(t *testing.T) {
			if _, err := CarregarConfig(env(vars)); err == nil {
				t.Fatalf("configuracao invalida (%v) foi aceita", vars)
			}
		})
	}
}
