package main

import (
	"testing"
	"time"
)

func env(pares map[string]string) ambiente {
	return func(chave string) string { return pares[chave] }
}

func TestConfigPadraoEUmaParceiraBemComportada(t *testing.T) {
	cfg, err := carregarConfig(env(nil))
	if err != nil {
		t.Fatalf("carregarConfig: %v", err)
	}

	if cfg.Nome != "partner" || cfg.Porta != "8080" {
		t.Errorf("nome %q e porta %q, esperados partner e 8080", cfg.Nome, cfg.Porta)
	}
	if cfg.Latencia != 0 || cfg.Jitter != 0 || cfg.TaxaFalha != 0 || cfg.DegradarApos != 0 {
		t.Errorf("sem configuracao a parceira deveria ser bem comportada, veio %+v", cfg)
	}
	if cfg.StatusFalha != 503 {
		t.Errorf("status de falha %d, esperado 503", cfg.StatusFalha)
	}
	if cfg.ValidadeCotacao != 300*time.Second {
		t.Errorf("validade da cotacao %s, esperada 5m", cfg.ValidadeCotacao)
	}
}

func TestConfigLeOPerfilCompleto(t *testing.T) {
	cfg, err := carregarConfig(env(map[string]string{
		"PARTNER_NAME":              "partner-degrading",
		"PORT":                      "9003",
		"PARTNER_SEED":              "7",
		"PARTNER_LATENCY_MS":        "120",
		"PARTNER_JITTER_MS":         "40",
		"PARTNER_FAILURE_RATE":      "0.25",
		"PARTNER_FAILURE_STATUS":    "502",
		"PARTNER_DEGRADE_AFTER":     "5",
		"PARTNER_DEGRADE_STEP_MS":   "300",
		"PARTNER_DEGRADE_CAP_MS":    "6000",
		"PARTNER_QUOTE_TTL_SECONDS": "60",
	}))
	if err != nil {
		t.Fatalf("carregarConfig: %v", err)
	}

	esperada := Config{
		Nome:            "partner-degrading",
		Porta:           "9003",
		Semente:         7,
		Latencia:        120 * time.Millisecond,
		Jitter:          40 * time.Millisecond,
		TaxaFalha:       0.25,
		StatusFalha:     502,
		DegradarApos:    5,
		DegradarPasso:   300 * time.Millisecond,
		DegradarTeto:    6 * time.Second,
		ValidadeCotacao: 60 * time.Second,
	}
	if cfg != esperada {
		t.Fatalf("config lida %+v, esperada %+v", cfg, esperada)
	}
}

func TestConfigInvalidaFalhaEmVezDeCairNoPadrao(t *testing.T) {
	casos := map[string]map[string]string{
		"taxa de falha acima de 1":      {"PARTNER_FAILURE_RATE": "40"},
		"taxa de falha negativa":        {"PARTNER_FAILURE_RATE": "-0.1"},
		"taxa de falha nao numerica":    {"PARTNER_FAILURE_RATE": "quarenta por cento"},
		"latencia negativa":             {"PARTNER_LATENCY_MS": "-1"},
		"latencia nao inteira":          {"PARTNER_LATENCY_MS": "150ms"},
		"status de falha fora da faixa": {"PARTNER_FAILURE_STATUS": "200"},
		"limiar de degradacao negativo": {"PARTNER_DEGRADE_AFTER": "-3"},
		"degradacao sem passo":          {"PARTNER_DEGRADE_AFTER": "5"},
		"validade zerada":               {"PARTNER_QUOTE_TTL_SECONDS": "0"},
		"semente nao numerica":          {"PARTNER_SEED": "abc"},
	}

	for nome, vars := range casos {
		t.Run(nome, func(t *testing.T) {
			if _, err := carregarConfig(env(vars)); err == nil {
				t.Fatalf("configuracao invalida (%v) foi aceita", vars)
			}
		})
	}
}
