package handlers

import (
	"encoding/json"
	"net/http"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

type LoginResponse struct {
	Token    string `json:"token"`
	Email    string `json:"email"`
	Username string `json:"username"`
}

type RegisterResponse struct {
	Token    string `json:"token"`
	Email    string `json:"email"`
	Username string `json:"username"`
}

func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, ErrorResponse{Error: message})
}
