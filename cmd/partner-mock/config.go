package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// Config e o comportamento ruim da parceira expresso como dado. Um unico binario serve os tres
// perfis do desafio (`partner-slow`, `partner-flaky`, `partner-degrading`) porque nada aqui e
// decidido em codigo: tudo entra por variavel de ambiente, e os perfis vivem no docker-compose.yml.
type Config struct {
	Nome            string        // PARTNER_NAME
	Porta           string        // PORT
	Semente         uint64        // PARTNER_SEED — governa toda a aleatoriedade aparente do mock
	Latencia        time.Duration // PARTNER_LATENCY_MS
	Jitter          time.Duration // PARTNER_JITTER_MS
	TaxaFalha       float64       // PARTNER_FAILURE_RATE (0 a 1)
	StatusFalha     int           // PARTNER_FAILURE_STATUS
	DegradarApos    int64         // PARTNER_DEGRADE_AFTER — requisicoes em voo toleradas sem degradar (0 desliga)
	DegradarPasso   time.Duration // PARTNER_DEGRADE_STEP_MS — latencia extra por requisicao em voo acima do limiar
	DegradarTeto    time.Duration // PARTNER_DEGRADE_CAP_MS — teto da degradacao (0 = sem teto)
	ValidadeCotacao time.Duration // PARTNER_QUOTE_TTL_SECONDS — quanto tempo a cotacao vale
}

// ambiente e a leitura de variaveis de ambiente como dependencia, para os testes nao precisarem
// mexer no processo.
type ambiente func(string) string

// carregarConfig monta a Config a partir do ambiente. Valor invalido e erro, nunca silenciosamente
// substituido pelo padrao: num starter didatico, um typo em `PARTNER_FAILURE_RATE` que vira 0 faz o
// aluno caçar por horas um circuit breaker que nunca abre.
func carregarConfig(env ambiente) (Config, error) {
	cfg := Config{
		Nome:  env.texto("PARTNER_NAME", "partner"),
		Porta: env.texto("PORT", "8080"),
	}

	var err error
	if cfg.Semente, err = env.inteiroPositivo("PARTNER_SEED", 20260729); err != nil {
		return Config{}, err
	}
	if cfg.Latencia, err = env.duracaoMs("PARTNER_LATENCY_MS", 0); err != nil {
		return Config{}, err
	}
	if cfg.Jitter, err = env.duracaoMs("PARTNER_JITTER_MS", 0); err != nil {
		return Config{}, err
	}
	if cfg.TaxaFalha, err = env.decimal("PARTNER_FAILURE_RATE", 0); err != nil {
		return Config{}, err
	}
	if cfg.DegradarPasso, err = env.duracaoMs("PARTNER_DEGRADE_STEP_MS", 0); err != nil {
		return Config{}, err
	}
	if cfg.DegradarTeto, err = env.duracaoMs("PARTNER_DEGRADE_CAP_MS", 0); err != nil {
		return Config{}, err
	}

	status, err := env.inteiro("PARTNER_FAILURE_STATUS", 503)
	if err != nil {
		return Config{}, err
	}
	cfg.StatusFalha = int(status)

	if cfg.DegradarApos, err = env.inteiro("PARTNER_DEGRADE_AFTER", 0); err != nil {
		return Config{}, err
	}

	ttl, err := env.inteiro("PARTNER_QUOTE_TTL_SECONDS", 300)
	if err != nil {
		return Config{}, err
	}
	cfg.ValidadeCotacao = time.Duration(ttl) * time.Second

	return cfg, cfg.validar()
}

func (c Config) validar() error {
	switch {
	case c.Nome == "":
		return fmt.Errorf("PARTNER_NAME nao pode ser vazio")
	case c.Porta == "":
		return fmt.Errorf("PORT nao pode ser vazio")
	case c.TaxaFalha < 0 || c.TaxaFalha > 1:
		return fmt.Errorf("PARTNER_FAILURE_RATE precisa estar entre 0 e 1, recebido %v", c.TaxaFalha)
	case c.StatusFalha < 400 || c.StatusFalha > 599:
		return fmt.Errorf("PARTNER_FAILURE_STATUS precisa ser um status de erro (400-599), recebido %d", c.StatusFalha)
	case c.DegradarApos < 0:
		return fmt.Errorf("PARTNER_DEGRADE_AFTER nao pode ser negativo, recebido %d", c.DegradarApos)
	case c.ValidadeCotacao <= 0:
		return fmt.Errorf("PARTNER_QUOTE_TTL_SECONDS precisa ser maior que zero")
	case c.DegradarApos > 0 && c.DegradarPasso <= 0:
		return fmt.Errorf("PARTNER_DEGRADE_AFTER exige PARTNER_DEGRADE_STEP_MS maior que zero")
	}
	return nil
}

// Resumo descreve o perfil ativo em uma linha, para o log de subida deixar obvio qual das tres
// parceiras esta de pe.
func (c Config) Resumo() string {
	resumo := fmt.Sprintf("latencia=%s jitter=%s falha=%.0f%%", c.Latencia, c.Jitter, c.TaxaFalha*100)
	if c.DegradarApos > 0 {
		resumo += fmt.Sprintf(" degrada=apos %d em voo, +%s cada (teto %s)", c.DegradarApos, c.DegradarPasso, c.DegradarTeto)
	}
	return resumo + fmt.Sprintf(" semente=%d", c.Semente)
}

// MarshalJSON expoe a configuracao efetiva em `GET /config`, com as duracoes em milissegundos —
// e o que o aluno le para conferir com que parametros a parceira esta rodando.
func (c Config) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Nome            string  `json:"partner"`
		Semente         uint64  `json:"seed"`
		LatenciaMs      int64   `json:"latency_ms"`
		JitterMs        int64   `json:"jitter_ms"`
		TaxaFalha       float64 `json:"failure_rate"`
		StatusFalha     int     `json:"failure_status"`
		DegradarApos    int64   `json:"degrade_after"`
		DegradarPassoMs int64   `json:"degrade_step_ms"`
		DegradarTetoMs  int64   `json:"degrade_cap_ms"`
		ValidadeSeg     int64   `json:"quote_ttl_seconds"`
	}{
		Nome:            c.Nome,
		Semente:         c.Semente,
		LatenciaMs:      c.Latencia.Milliseconds(),
		JitterMs:        c.Jitter.Milliseconds(),
		TaxaFalha:       c.TaxaFalha,
		StatusFalha:     c.StatusFalha,
		DegradarApos:    c.DegradarApos,
		DegradarPassoMs: c.DegradarPasso.Milliseconds(),
		DegradarTetoMs:  c.DegradarTeto.Milliseconds(),
		ValidadeSeg:     int64(c.ValidadeCotacao.Seconds()),
	})
}

func (a ambiente) texto(chave, padrao string) string {
	if v := a(chave); v != "" {
		return v
	}
	return padrao
}

func (a ambiente) inteiro(chave string, padrao int64) (int64, error) {
	v := a(chave)
	if v == "" {
		return padrao, nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s precisa ser um inteiro, recebido %q", chave, v)
	}
	return n, nil
}

func (a ambiente) inteiroPositivo(chave string, padrao uint64) (uint64, error) {
	v := a(chave)
	if v == "" {
		return padrao, nil
	}
	n, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s precisa ser um inteiro nao negativo, recebido %q", chave, v)
	}
	return n, nil
}

func (a ambiente) decimal(chave string, padrao float64) (float64, error) {
	v := a(chave)
	if v == "" {
		return padrao, nil
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, fmt.Errorf("%s precisa ser um numero, recebido %q", chave, v)
	}
	return n, nil
}

func (a ambiente) duracaoMs(chave string, padraoMs int64) (time.Duration, error) {
	ms, err := a.inteiro(chave, padraoMs)
	if err != nil {
		return 0, err
	}
	if ms < 0 {
		return 0, fmt.Errorf("%s nao pode ser negativo, recebido %d", chave, ms)
	}
	return time.Duration(ms) * time.Millisecond, nil
}
