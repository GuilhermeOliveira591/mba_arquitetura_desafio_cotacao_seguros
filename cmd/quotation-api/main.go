// Command quotation-api e a API de cotacao de seguros multi-tenant do starter.
//
// Neste ponto o binario existe para provar o esqueleto: modulo, layout e build. O contrato
// HTTP de verdade — cotacao multi-tenant e agregacao sincrona das seguradoras parceiras —
// entra em cima deste mesmo main.
package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	addr := ":" + porta()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{"status":"ok"}` + "\n")); err != nil {
			log.Printf("falha ao escrever resposta do healthcheck: %v", err)
		}
	})

	log.Printf("quotation-api ouvindo em %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("servidor encerrou: %v", err)
	}
}

// porta le a porta de escuta do ambiente, com 8080 como padrao.
func porta() string {
	if p := os.Getenv("PORT"); p != "" {
		return p
	}
	return "8080"
}
