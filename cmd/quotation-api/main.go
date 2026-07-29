// Command quotation-api is the multi-tenant insurance quotation API of the starter.
//
// It takes a quote request from a broker and calls the partner insurers, aggregating the responses.
// It works — and it works badly on purpose: the partners are called serially, with no timeout, no
// circuit breaker, no cache and no fallback. That emptiness is the challenge statement, not an
// oversight; see internal/quotation/service.go and internal/partner/client.go.
//
// Contract:
//
//	POST /quotes   aggregated quote from the partners (requires the X-Tenant-Id header)
//	GET  /healthz  health of the process
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
	cfg, err := platform.LoadConfig(platform.Environment())
	if err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}

	api := quotation.NewAPI(
		quotation.NewService(cfg.Partners, partner.NewClient()),
		cfg.Tenants,
	)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           api.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stopListening := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopListening()

	go func() {
		<-ctx.Done()
		log.Print("quotation-api: signal received, shutting down")
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			log.Printf("quotation-api: forced shutdown: %v", err)
		}
	}()

	log.Printf("quotation-api listening on %s | partners=%d brokers=%v",
		server.Addr, len(cfg.Partners), cfg.Tenants)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server stopped: %v", err)
	}
}
