package main

import (
	"errors"
	"net/http"
)

type StaticHandler struct {
	Storage  ObjectStore
	Paths    StaticPaths
	Security SecurityHeaders
}

func (handler StaticHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	handler.Security.Apply(handler.reply(request)).Send(writer)
}

func (handler StaticHandler) reply(request *http.Request) Reply {
	reply, err := handler.fetch(request)
	if err != nil {
		return Problems{}.Reply(err)
	}
	return reply
}

func (handler StaticHandler) fetch(request *http.Request) (Reply, error) {
	if err := handler.checkMethod(request.Method); err != nil {
		return Reply{}, err
	}
	if err := handler.Paths.Check(request.URL.Path); err != nil {
		return Reply{}, err
	}
	reply, err := handler.Storage.Get(request.Context(), handler.Paths.Key(request.URL.Path))
	return handler.withFallback(request, reply, err)
}

func (StaticHandler) checkMethod(method string) error {
	if method != http.MethodGet && method != http.MethodHead {
		return Problems{}.MethodNotAllowed()
	}
	return nil
}

func (handler StaticHandler) withFallback(request *http.Request, reply Reply, err error) (Reply, error) {
	if handler.fallsBack(request, err) {
		return handler.Storage.Get(request.Context(), handler.Paths.Index(request.URL.Path))
	}
	return reply, err
}

func (handler StaticHandler) fallsBack(request *http.Request, err error) bool {
	var problem Problem
	return errors.As(err, &problem) && problem.Status == http.StatusNotFound && handler.Paths.PageLike(request.URL.Path, request.Header.Get("Accept"))
}
