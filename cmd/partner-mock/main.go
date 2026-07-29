// Command partner-mock e a seguradora parceira falsa do starter — a dependencia externa instavel
// contra a qual o aluno vai construir circuit breaker, cache e fallback.
//
// E um binario so para os tres perfis do desafio. O que muda entre `partner-slow`, `partner-flaky` e
// `partner-degrading` e configuracao, nao codigo: o mau comportamento mora no docker-compose.yml,
// em um lugar unico e legivel, e o starter continua pequeno.
//
// Contrato:
//
//	POST /quotes   cotacao da parceira (aplica latencia, falha e degradacao do perfil)
//	GET  /healthz  saude do processo — nunca degrada, nunca falha
//	GET  /config   configuracao efetiva do perfil que esta rodando
package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

// limitePedido corta pedidos absurdos antes de eles virarem memoria. O mock nao valida o conteudo do
// pedido de proposito: quem cuida de tenant e de payload e a quotation-api, nao a parceira.
const limitePedido = 1 << 20 // 1 MiB

func main() {
	cfg, err := carregarConfig(os.Getenv)
	if err != nil {
		log.Fatalf("configuracao invalida: %v", err)
	}

	comportamento := NovoComportamento(cfg)
	servidor := &http.Server{
		Addr:              ":" + cfg.Porta,
		Handler:           rotas(cfg, comportamento),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, pararEscuta := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer pararEscuta()

	go func() {
		<-ctx.Done()
		log.Printf("%s: sinal recebido, encerrando", cfg.Nome)
		desligamento, cancelar := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelar()
		if err := servidor.Shutdown(desligamento); err != nil {
			log.Printf("%s: desligamento forcado: %v", cfg.Nome, err)
		}
	}()

	log.Printf("%s ouvindo em %s | %s", cfg.Nome, servidor.Addr, cfg.Resumo())
	if err := servidor.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("servidor encerrou: %v", err)
	}
}

func rotas(cfg Config, comportamento *Comportamento) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /quotes", cotar(cfg, comportamento))
	mux.HandleFunc("GET /healthz", saude(cfg))
	mux.HandleFunc("GET /config", configuracao(cfg))
	return mux
}

// cotar e o unico endpoint que sofre o perfil: aplica a latencia decidida na chegada e so entao
// responde — sucesso ou falha. Falhar depois de esperar e o caso ruim de verdade, o que consome o
// timeout do cliente; falha instantanea seria facil demais de tolerar.
func cotar(cfg Config, comportamento *Comportamento) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		decisao := comportamento.Admitir()
		defer comportamento.Concluir()

		pedido, err := io.ReadAll(http.MaxBytesReader(w, r.Body, limitePedido))
		if err != nil {
			responder(w, cfg, decisao, http.StatusRequestEntityTooLarge, erroJSON{
				Erro:     "pedido excede o limite aceito pela parceira",
				Parceira: cfg.Nome,
			})
			return
		}

		if err := dormir(r.Context(), decisao.Latencia); err != nil {
			// Cliente desistiu (timeout ou cancelamento) antes de a parceira responder. Nao ha para
			// quem escrever: so libera a vaga pelo defer.
			return
		}

		if decisao.Falha {
			responder(w, cfg, decisao, cfg.StatusFalha, erroJSON{
				Erro:     "parceira indisponivel",
				Parceira: cfg.Nome,
			})
			return
		}

		responder(w, cfg, decisao, http.StatusOK, comportamento.Cotar(pedido))
	}
}

// saude responde sempre, na hora. O healthcheck do compose nao pode enxergar o mau comportamento do
// perfil, senao a parceira lenta jamais subiria "healthy" e o ambiente nao ficaria de pe.
func saude(cfg Config) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		escreverJSON(w, http.StatusOK, map[string]string{"status": "ok", "partner": cfg.Nome})
	}
}

func configuracao(cfg Config) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		escreverJSON(w, http.StatusOK, cfg)
	}
}

type erroJSON struct {
	Erro     string `json:"error"`
	Parceira string `json:"partner"`
}

// responder anexa a decisao do mock aos cabecalhos da resposta. E o atalho de diagnostico do starter:
// da para ver o numero de sequencia, a latencia aplicada e a carga do momento sem abrir o Jaeger —
// util justamente antes de o aluno instrumentar.
func responder(w http.ResponseWriter, cfg Config, decisao Decisao, status int, corpo any) {
	cabecalho := w.Header()
	cabecalho.Set("X-Partner-Name", cfg.Nome)
	cabecalho.Set("X-Partner-Seq", strconv.FormatUint(decisao.Sequencia, 10))
	cabecalho.Set("X-Partner-Latency-Ms", strconv.FormatInt(decisao.Latencia.Milliseconds(), 10))
	cabecalho.Set("X-Partner-Inflight", strconv.FormatInt(decisao.EmVoo, 10))
	escreverJSON(w, status, corpo)
}

func escreverJSON(w http.ResponseWriter, status int, corpo any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(corpo); err != nil {
		log.Printf("falha ao escrever resposta: %v", err)
	}
}

// dormir espera respeitando o cancelamento do cliente — sem isso, uma parceira lenta seguraria
// goroutines de requisicoes que ja foram embora.
func dormir(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	temporizador := time.NewTimer(d)
	defer temporizador.Stop()
	select {
	case <-temporizador.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
