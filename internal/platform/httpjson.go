package platform

import (
	"encoding/json"
	"log"
	"net/http"
)

// ErrorBody is the standard error body of the API.
type ErrorBody struct {
	Message string `json:"error"`
	Partner string `json:"partner,omitempty"`
}

// WriteJSON replies with the body serialized as JSON.
func WriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("failed to write response: %v", err)
	}
}

// WriteError replies with the standard error body.
func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, ErrorBody{Message: message})
}
