package platform

import (
	"encoding/json"
	"log"
	"net/http"
)

// Erro e o corpo padrao de erro da API.
type Erro struct {
	Mensagem string `json:"error"`
	Parceira string `json:"partner,omitempty"`
}

// EscreverJSON responde com o corpo serializado em JSON.
func EscreverJSON(w http.ResponseWriter, status int, corpo any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(corpo); err != nil {
		log.Printf("falha ao escrever resposta: %v", err)
	}
}

// EscreverErro responde com o corpo de erro padrao.
func EscreverErro(w http.ResponseWriter, status int, mensagem string) {
	EscreverJSON(w, status, Erro{Mensagem: mensagem})
}
