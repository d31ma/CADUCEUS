package main

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

type KeycloakProxy struct {
	Target *url.URL
}

func (proxy KeycloakProxy) Handler() http.Handler {
	return &httputil.ReverseProxy{Rewrite: proxy.rewrite}
}

func (proxy KeycloakProxy) rewrite(request *httputil.ProxyRequest) {
	request.SetURL(proxy.Target)
	request.SetXForwarded()
	request.Out.Host = request.In.Host
}
