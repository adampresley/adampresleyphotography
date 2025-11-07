package main

import (
	"context"
	"net/http"
	"strings"

	"github.com/adampresley/adamgokit/sessions"
	"github.com/adampresley/adampresleyphotography/pkg/models"
)

func newClientAccessMiddleware(sessionService sessions.Session[*models.Client], excludedPaths []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var (
				err           error
				sessionClient *models.Client
			)

			path := r.URL.Path

			/*
			 * If this path is excluded, keep going.
			 */
			for _, excludedPath := range excludedPaths {
				if strings.HasPrefix(path, excludedPath) {
					next.ServeHTTP(w, r)
					return
				}
			}

			if sessionClient, err = sessionService.Get(r); err != nil {
				http.Redirect(w, r, "/client/login", http.StatusTemporaryRedirect)
				return
			}

			ctx := context.WithValue(r.Context(), "client", sessionClient)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
