package api

import (
	"context"
	"net/http"
	"time"

	"github.com/dlmiddlecote/kit/api"
)

func timeout(d time.Duration) api.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get context with timeout.
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()

			// Call next handler with the timeout context.
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
