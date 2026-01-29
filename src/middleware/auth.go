package middleware

import (
	"context"
	"net/http"

	"github.com/bamdadam/backend/src/model"
)

const AuthHeader string = "X-User-ID"

func Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get(AuthHeader)
		if userID == "" {
			http.Error(w, `{"error":"X-User-ID header is required"}`, http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), model.UserIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
