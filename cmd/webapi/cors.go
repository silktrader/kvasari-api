package main

import (
	"github.com/gorilla/handlers"
	"net/http"
)

// applyCORSHandler applies a CORS policy to the router.
func applyCORSHandler(h http.Handler) http.Handler {
	return handlers.CORS(
		handlers.AllowedHeaders([]string{
			"Content-Type", "Authorization",
		}),
		handlers.AllowCredentials(),
		handlers.AllowedMethods([]string{"GET", "POST", "OPTIONS", "DELETE", "PUT"}),
		handlers.AllowedOrigins([]string{"*"}),
	)(h)
}
