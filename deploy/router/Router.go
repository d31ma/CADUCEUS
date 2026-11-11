package main

import (
	"net/http"
	"strings"
)

type Router struct {
	Ready        http.Handler
	Blocked      BlockedPaths
	Passthroughs Passthroughs
	Blobs        http.Handler
	Api          http.Handler
	Keycloak     http.Handler
	Static       http.Handler
}

func (router Router) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	router.handlerFor(request.URL.Path).ServeHTTP(writer, request)
}

func (router Router) handlerFor(path string) http.Handler {
	switch {
	case path == "/_router/ready":
		return router.Ready
	case router.Blocked.Covers(path):
		return NotFoundHandler{}
	case router.Passthroughs.Has(path):
		return router.Passthroughs.For(path)
	case strings.HasPrefix(path, router.blobsPrefix()+"/"):
		return router.Blobs
	case strings.HasPrefix(path, Routes{}.ApiPrefix()):
		return router.Api
	case strings.HasPrefix(path, "/idp/"):
		return router.Keycloak
	default:
		return router.Static
	}
}

func (Router) blobsPrefix() string {
	return "/blobs"
}
