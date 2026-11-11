package main

import (
	"cmp"
	"net/http"
	"net/http/httputil"
	"net/url"
	"slices"
	"strings"
)

type Passthroughs struct {
	Routes []PrefixRoute
}

func (passthroughs Passthroughs) Has(path string) bool {
	return passthroughs.index(path) >= 0
}

func (passthroughs Passthroughs) For(path string) http.Handler {
	return passthroughs.Routes[passthroughs.index(path)].Handler
}

func (passthroughs Passthroughs) index(path string) int {
	return slices.IndexFunc(passthroughs.Routes, func(route PrefixRoute) bool { return strings.HasPrefix(path, route.Prefix) })
}

func (passthroughs Passthroughs) Of(targets map[string]string) (Passthroughs, error) {
	routes, err := passthroughs.routes(targets)
	return Passthroughs{Routes: passthroughs.longestFirst(routes)}, err
}

func (Passthroughs) longestFirst(routes []PrefixRoute) []PrefixRoute {
	slices.SortStableFunc(routes, func(first PrefixRoute, second PrefixRoute) int {
		return cmp.Compare(len(second.Prefix), len(first.Prefix))
	})
	return routes
}

func (passthroughs Passthroughs) routes(targets map[string]string) ([]PrefixRoute, error) {
	routes := []PrefixRoute{}
	for prefix, address := range targets {
		proxy, err := passthroughs.proxy(address)
		if err != nil {
			return nil, err
		}
		routes = append(routes, PrefixRoute{Prefix: prefix, Handler: proxy})
	}
	return routes, nil
}

func (Passthroughs) proxy(address string) (http.Handler, error) {
	target, err := url.Parse(address)
	if err != nil {
		return nil, InvalidSetting{Name: "MESAIL_ROUTER_PASSTHROUGH", Reason: err.Error()}
	}
	return &httputil.ReverseProxy{Rewrite: UpstreamTarget{Target: target}.Rewrite, FlushInterval: -1}, nil
}
