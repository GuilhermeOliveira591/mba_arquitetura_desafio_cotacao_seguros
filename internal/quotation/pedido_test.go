package quotation

import "testing"

func TestNormalizarPreencheOsPadroesEArruma(t *testing.T) {
	pedido := Pedido{
		Condutor: Condutor{Documento: "  12345678901 ", AnoNascimento: 1988},
		Veiculo:  Veiculo{Placa: " abc1d23 ", Modelo: " Gol 1.0 ", Ano: 2020, ValorCentavos: 8500000},
	}

	if err := pedido.Normalizar(); err != nil {
		t.Fatalf("Normalizar: %v", err)
	}
	if pedido.Cobertura != coberturaPadrao {
		t.Errorf("cobertura %q, esperada %q", pedido.Cobertura, coberturaPadrao)
	}
	if pedido.Veiculo.Placa != "ABC1D23" {
		t.Errorf("placa %q, esperada ABC1D23", pedido.Veiculo.Placa)
	}
	if pedido.Condutor.Documento != "12345678901" {
		t.Errorf("documento %q, esperado sem espacos", pedido.Condutor.Documento)
	}
}

func TestNormalizarRejeitaPedidoIncompleto(t *testing.T) {
	completo := func() Pedido {
		return Pedido{
			Condutor: Condutor{Documento: "12345678901", AnoNascimento: 1988},
			Veiculo:  Veiculo{Placa: "ABC1D23", Ano: 2020, ValorCentavos: 8500000},
		}
	}

	casos := map[string]func(*Pedido){
		"sem documento":         func(p *Pedido) { p.Condutor.Documento = "" },
		"sem ano de nascimento": func(p *Pedido) { p.Condutor.AnoNascimento = 0 },
		"sem placa":             func(p *Pedido) { p.Veiculo.Placa = "" },
		"sem ano do veiculo":    func(p *Pedido) { p.Veiculo.Ano = 0 },
		"valor zerado":          func(p *Pedido) { p.Veiculo.ValorCentavos = 0 },
		"valor negativo":        func(p *Pedido) { p.Veiculo.ValorCentavos = -1 },
		"cobertura invalida":    func(p *Pedido) { p.Cobertura = "vip" },
	}

	for nome, quebrar := range casos {
		t.Run(nome, func(t *testing.T) {
			pedido := completo()
			quebrar(&pedido)
			if err := pedido.Normalizar(); err == nil {
				t.Fatalf("pedido invalido foi aceito: %+v", pedido)
			}
		})
	}
}
