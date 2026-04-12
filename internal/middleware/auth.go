package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/wasp/helixtrace-api/internal/handlers"
)

func AuthMiddleware(conn clickhouse.Conn, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			handlers.WriteError(w, http.StatusUnauthorized, "missing authorization header")
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == authHeader {
			handlers.WriteError(w, http.StatusUnauthorized, "invalid authorization header format")
			return
		}

		var email string
		err := conn.QueryRow(r.Context(), `
			SELECT email FROM tokens FINAL
			WHERE token = ? AND expires_at > now64()
		`, token).Scan(&email)
		if err != nil {
			handlers.WriteError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		ctx := r.Context()
		ctx = ContextWithEmail(ctx, email)
		next(w, r.WithContext(ctx))
	}
}

type contextKey string

const emailKey contextKey = "email"

func ContextWithEmail(ctx context.Context, email string) context.Context {
	return context.WithValue(ctx, emailKey, email)
}

func EmailFromContext(ctx context.Context) (string, bool) {
	email, ok := ctx.Value(emailKey).(string)
	return email, ok
}
