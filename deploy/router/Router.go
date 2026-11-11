package main

import (
	"net/http"
	"strings"
)

type Router struct {
	Ready    http.Handler
	Api      http.Handler
	Keycloak http.Handler
	Static   http.Handler
}

func (router Router) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	router.handlerFor(request.URL.Path).ServeHTTP(writer, request)
}

func (router Router) handlerFor(path string) http.Handler {
	switch {
	case path == "/_router/ready":
		return router.Ready
	case strings.HasPrefix(path, Routes{}.ApiPrefix()):
		return router.Api
	case strings.HasPrefix(path, "/realms/"), strings.HasPrefix(path, "/resources/"):
		return router.Keycloak
	default:
		return router.Static
	}
}
