package main

import (
	"testing"
	"time"
)

// perfilFlaky reproduz o `partner-flaky` do docker-compose.yml. Os testes deste arquivo sao a defesa
// contra o risco registrado na spec ("mocks instaveis serem instaveis demais — ou de menos"): se
// alguem mexer nos defaults e quebrar a reprodutibilidade ou as rajadas de falha, quebra aqui.
func perfilFlaky() Config {
	return Config{
		Nome:            "partner-flaky",
		Porta:           "8080",
		Semente:         20260729,
		Latencia:        150 * time.Millisecond,
		Jitter:          50 * time.Millisecond,
		TaxaFalha:       0.4,
		StatusFalha:     503,
		ValidadeCotacao: 300 * time.Second,
	}
}

func sequenciaDeFalhas(cfg Config, n int) []bool {
	c := NovoComportamento(cfg)
	falhas := make([]bool, n)
	for i := range falhas {
		d := c.Admitir()
		c.Concluir()
		falhas[i] = d.Falha
	}
	return falhas
}

func TestFalhasSaoIdenticasEntreExecucoes(t *testing.T) {
	primeira := sequenciaDeFalhas(perfilFlaky(), 500)
	segunda := sequenciaDeFalhas(perfilFlaky(), 500)

	for i := range primeira {
		if primeira[i] != segunda[i] {
			t.Fatalf("requisicao %d divergiu entre execucoes: %v e %v", i+1, primeira[i], segunda[i])
		}
	}
}

func TestSementeDiferenteMudaASequencia(t *testing.T) {
	cfg := perfilFlaky()
	padrao := sequenciaDeFalhas(cfg, 200)
	cfg.Semente = 1
	outra := sequenciaDeFalhas(cfg, 200)

	for i := range padrao {
		if padrao[i] != outra[i] {
			return
		}
	}
	t.Fatal("sementes diferentes produziram a mesma sequencia de falhas")
}

func TestTaxaDeFalhaObservadaFicaPertoDaConfigurada(t *testing.T) {
	const amostras = 20000
	falhas := 0
	for _, f := range sequenciaDeFalhas(perfilFlaky(), amostras) {
		if f {
			falhas++
		}
	}

	observada := float64(falhas) / amostras
	if diferenca := observada - perfilFlaky().TaxaFalha; diferenca > 0.02 || diferenca < -0.02 {
		t.Fatalf("taxa de falha observada %.4f, esperada %.2f (tolerancia 0.02)", observada, perfilFlaky().TaxaFalha)
	}
}

// TestPerfilFlakyProduzRajadas garante que o circuit breaker do aluno tem como abrir. Um mock que
// alternasse sucesso e falha de forma regular nunca dispararia um breaker que conta falhas
// consecutivas — e o exercicio inteiro do desafio morreria no default.
func TestPerfilFlakyProduzRajadas(t *testing.T) {
	const janela = 200
	const rajadaMinima = 4

	maior, corrente := 0, 0
	for _, falhou := range sequenciaDeFalhas(perfilFlaky(), janela) {
		if !falhou {
			corrente = 0
			continue
		}
		corrente++
		if corrente > maior {
			maior = corrente
		}
	}

	if maior < rajadaMinima {
		t.Fatalf("maior rajada de falhas nas primeiras %d requisicoes foi %d, esperado ao menos %d", janela, maior, rajadaMinima)
	}
}

func TestTaxaZeroNuncaFalhaETaxaUmSempreFalha(t *testing.T) {
	cfg := perfilFlaky()
	cfg.TaxaFalha = 0
	for i, falhou := range sequenciaDeFalhas(cfg, 100) {
		if falhou {
			t.Fatalf("taxa 0 falhou na requisicao %d", i+1)
		}
	}

	cfg.TaxaFalha = 1
	for i, falhou := range sequenciaDeFalhas(cfg, 100) {
		if !falhou {
			t.Fatalf("taxa 1 teve sucesso na requisicao %d", i+1)
		}
	}
}

func TestJitterFicaDentroDaFaixa(t *testing.T) {
	cfg := perfilFlaky()
	c := NovoComportamento(cfg)

	viuVariacao := false
	for i := 0; i < 500; i++ {
		d := c.Admitir()
		c.Concluir()
		if d.Latencia < cfg.Latencia || d.Latencia >= cfg.Latencia+cfg.Jitter {
			t.Fatalf("latencia %s fora da faixa [%s, %s)", d.Latencia, cfg.Latencia, cfg.Latencia+cfg.Jitter)
		}
		if d.Latencia != cfg.Latencia {
			viuVariacao = true
		}
	}
	if !viuVariacao {
		t.Fatal("jitter configurado mas nenhuma latencia variou")
	}
}

func TestDegradacaoSoComecaAcimaDoLimiar(t *testing.T) {
	cfg := Config{
		Nome:            "partner-degrading",
		Semente:         1,
		Latencia:        120 * time.Millisecond,
		DegradarApos:    5,
		DegradarPasso:   300 * time.Millisecond,
		DegradarTeto:    6 * time.Second,
		ValidadeCotacao: 300 * time.Second,
	}
	c := NovoComportamento(cfg)

	casos := []struct {
		emVoo    int64
		esperado time.Duration
	}{
		{1, 0},
		{5, 0},
		{6, 300 * time.Millisecond},
		{10, 1500 * time.Millisecond},
		{25, 6 * time.Second},  // teto
		{100, 6 * time.Second}, // teto se mantem
	}

	for _, caso := range casos {
		if obtido := c.degradacao(caso.emVoo); obtido != caso.esperado {
			t.Errorf("com %d em voo: degradacao %s, esperada %s", caso.emVoo, obtido, caso.esperado)
		}
	}
}

func TestDegradacaoDesligadaQuandoLimiarEZero(t *testing.T) {
	c := NovoComportamento(Config{Latencia: 100 * time.Millisecond, DegradarPasso: time.Second})
	if d := c.degradacao(1000); d != 0 {
		t.Fatalf("degradacao %s com PARTNER_DEGRADE_AFTER=0, esperada 0", d)
	}
}

func TestAdmitirContaRequisicoesEmVoo(t *testing.T) {
	c := NovoComportamento(perfilFlaky())

	primeira := c.Admitir()
	segunda := c.Admitir()
	if primeira.EmVoo != 1 || segunda.EmVoo != 2 {
		t.Fatalf("em voo na chegada: %d e %d, esperados 1 e 2", primeira.EmVoo, segunda.EmVoo)
	}
	if primeira.Sequencia != 1 || segunda.Sequencia != 2 {
		t.Fatalf("sequencia: %d e %d, esperadas 1 e 2", primeira.Sequencia, segunda.Sequencia)
	}

	c.Concluir()
	c.Concluir()
	if restante := c.EmVoo(); restante != 0 {
		t.Fatalf("%d requisicoes em voo depois de concluir todas", restante)
	}
}

func TestCotacaoEEstavelPorPedido(t *testing.T) {
	c := NovoComportamento(perfilFlaky())
	pedido := []byte(`{"tenant_id":"corretora-a","driver_age":35}`)

	primeira := c.Cotar(pedido)
	segunda := c.Cotar(pedido)
	if primeira != segunda {
		t.Fatalf("mesmo pedido gerou cotacoes diferentes: %+v e %+v", primeira, segunda)
	}
	if outra := c.Cotar([]byte(`{"tenant_id":"corretora-b","driver_age":35}`)); outra == primeira {
		t.Fatal("pedidos diferentes geraram a mesma cotacao")
	}

	if primeira.Parceira != "partner-flaky" {
		t.Errorf("parceira %q, esperada partner-flaky", primeira.Parceira)
	}
	if primeira.Moeda != "BRL" {
		t.Errorf("moeda %q, esperada BRL", primeira.Moeda)
	}
	if primeira.PremioCentavos < 50000 || primeira.PremioCentavos > 200000 {
		t.Errorf("premio %d centavos fora da faixa esperada", primeira.PremioCentavos)
	}
	if primeira.ValidadeSegundos != 300 {
		t.Errorf("validade %d segundos, esperada 300", primeira.ValidadeSegundos)
	}
}

// TestParceirasDiferentesCotamDiferente garante que a agregacao da quotation-api tem o que comparar:
// tres parceiras devolvendo o mesmo premio para o mesmo pedido esvaziariam o cenario.
func TestParceirasDiferentesCotamDiferente(t *testing.T) {
	pedido := []byte(`{"tenant_id":"corretora-a"}`)
	premios := map[int64]string{}

	for _, nome := range []string{"partner-slow", "partner-flaky", "partner-degrading"} {
		cfg := perfilFlaky()
		cfg.Nome = nome
		cotacao := NovoComportamento(cfg).Cotar(pedido)
		if anterior, repetido := premios[cotacao.PremioCentavos]; repetido {
			t.Fatalf("%s e %s cotaram o mesmo premio %d", anterior, nome, cotacao.PremioCentavos)
		}
		premios[cotacao.PremioCentavos] = nome
	}
}
