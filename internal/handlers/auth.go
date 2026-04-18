package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/wasp/helixtrace-api/internal/models"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	Conn clickhouse.Conn
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "failed to read request body")
		return
	}
	defer r.Body.Close()

	var req loginRequest
	if err := json.Unmarshal(body, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if req.Email == "" || req.Password == "" {
		WriteError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	var user models.User
	err = h.Conn.QueryRow(r.Context(), `
		SELECT email, password_hash, username, active, access_rights
		FROM users FINAL
		WHERE email = ?
	`, req.Email).Scan(
		&user.Email, &user.PasswordHash, &user.Username, &user.Active, &user.AccessRights,
	)
	if err != nil {
		WriteError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		WriteError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	if !user.Active {
		WriteError(w, http.StatusForbidden, "account is disabled")
		return
	}

	token, err := generateToken()
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	now := time.Now().UTC()
	expiresAt := now.Add(24 * time.Hour)

	if err := h.Conn.Exec(r.Context(), `
		INSERT INTO tokens (token, email, created_at, expires_at)
		VALUES (?, ?, ?, ?)
	`, token, user.Email, now, expiresAt); err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	WriteJSON(w, http.StatusOK, LoginResponse{
		Token:    token,
		Email:    user.Email,
		Username: user.Username,
	})
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "failed to read request body")
		return
	}
	defer r.Body.Close()

	var req registerRequest
	if err := json.Unmarshal(body, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if req.Email == "" || req.Password == "" {
		WriteError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	var existingCount uint64
	err = h.Conn.QueryRow(r.Context(), `
		SELECT count() FROM users FINAL WHERE email = ?
	`, req.Email).Scan(&existingCount)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to check user")
		return
	}
	if existingCount > 0 {
		WriteError(w, http.StatusConflict, "user already exists")
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	now := time.Now().UTC()
	if err := h.Conn.Exec(r.Context(), `
		INSERT INTO users (email, password_hash, username, active, access_rights, updated_at)
		VALUES (?, ?, ?, true, '', ?)
	`, req.Email, string(hashedPassword), req.Email, now); err != nil {
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create user: %v", err))
		return
	}

	token, err := generateToken()
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	expiresAt := now.Add(24 * time.Hour)
	if err := h.Conn.Exec(r.Context(), `
		INSERT INTO tokens (token, email, created_at, expires_at)
		VALUES (?, ?, ?, ?)
	`, token, req.Email, now, expiresAt); err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	WriteJSON(w, http.StatusCreated, RegisterResponse{
		Token:    token,
		Email:    req.Email,
		Username: req.Email,
	})
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
