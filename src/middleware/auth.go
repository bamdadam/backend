package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/bamdadam/backend/src/model"
)

const AuthHeader string = "X-User-ID"

func Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isWebSocketUpgrade(r) {
			next.ServeHTTP(w, r)
			return
		}

		userID := r.Header.Get(AuthHeader)
		if userID == "" {
			http.Error(w, `{"error":"X-User-ID header is required"}`, http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), model.UserIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func isWebSocketUpgrade(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Connection"), "Upgrade") &&
		strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}
