package main

import (
	"maps"
	"net/http"
	"net/http/httputil"
	"net/url"
	"slices"
)

type UpstreamHandler struct {
	Routes  Routes
	Proxies map[string]http.Handler
}

func (handler UpstreamHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	handler.handlerFor(request.URL.Path).ServeHTTP(writer, request)
}

func (handler UpstreamHandler) handlerFor(path string) http.Handler {
	service, err := handler.Routes.Service(path)
	if err != nil {
		return ProblemHandler{Err: err}
	}
	return handler.Proxies[service]
}

func (handler UpstreamHandler) Of(upstreams map[string]string) (UpstreamHandler, error) {
	proxies, err := handler.proxies(upstreams)
	if err != nil {
		return UpstreamHandler{}, err
	}
	return UpstreamHandler{Routes: Routes{}.Of(slices.Collect(maps.Keys(proxies)), ""), Proxies: proxies}, nil
}

func (handler UpstreamHandler) proxies(upstreams map[string]string) (map[string]http.Handler, error) {
	proxies := map[string]http.Handler{}
	for service, address := range upstreams {
		proxy, err := handler.proxy(address)
		if err != nil {
			return nil, err
		}
		proxies[service] = proxy
	}
	return proxies, nil
}

func (UpstreamHandler) proxy(address string) (http.Handler, error) {
	target, err := url.Parse(address)
	if err != nil {
		return nil, InvalidSetting{Name: "MESAIL_ROUTER_UPSTREAMS", Reason: err.Error()}
	}
	return &httputil.ReverseProxy{Rewrite: UpstreamTarget{Target: target}.Rewrite}, nil
}
