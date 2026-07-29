// Command quotation-api e a API de cotacao de seguros multi-tenant do starter.
//
// Ela recebe um pedido de cotacao de uma corretora e consulta as seguradoras parceiras, agregando as
// respostas. Funciona — e funciona mal de proposito: as parceiras sao consultadas em serie, sem
// timeout, sem circuit breaker, sem cache e sem fallback. Esse vazio e o enunciado do desafio, nao um
// esquecimento; ver internal/quotation/servico.go e internal/partner/client.go.
//
// Contrato:
//
//	POST /quotes   cotacao agregada das parceiras (exige o cabecalho X-Tenant-Id)
//	GET  /healthz  saude do processo
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/GuilhermeOliveira591/mba_arquitetura_desafio_cotacao_seguros/internal/partner"
	"github.com/GuilhermeOliveira591/mba_arquitetura_desafio_cotacao_seguros/internal/platform"
	"github.com/GuilhermeOliveira591/mba_arquitetura_desafio_cotacao_seguros/internal/quotation"
)

func main() {
	cfg, err := platform.CarregarConfig(platform.Ambiente())
	if err != nil {
		log.Fatalf("configuracao invalida: %v", err)
	}

	api := quotation.NovaAPI(
		quotation.NovoServico(cfg.Parceiras, partner.NovoCliente()),
		cfg.Tenants,
	)

	servidor := &http.Server{
		Addr:              ":" + cfg.Porta,
		Handler:           api.Rotas(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, pararEscuta := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer pararEscuta()

	go func() {
		<-ctx.Done()
		log.Print("quotation-api: sinal recebido, encerrando")
		desligamento, cancelar := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelar()
		if err := servidor.Shutdown(desligamento); err != nil {
			log.Printf("quotation-api: desligamento forcado: %v", err)
		}
	}()

	log.Printf("quotation-api ouvindo em %s | parceiras=%d corretoras=%v",
		servidor.Addr, len(cfg.Parceiras), cfg.Tenants)
	if err := servidor.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("servidor encerrou: %v", err)
	}
}
