package main

import "strings"

type Routes struct {
	Known  map[string]bool
	Prefix string
}

func (Routes) ApiPrefix() string {
	return "/api/v1/"
}

func (routes Routes) Function(path string) (string, error) {
	service, err := routes.Service(path)
	if err != nil {
		return "", err
	}
	return routes.Prefix + service, nil
}

func (routes Routes) Service(path string) (string, error) {
	service, _, _ := strings.Cut(strings.TrimPrefix(path, routes.ApiPrefix()), "/")
	return routes.known(service)
}

func (routes Routes) known(service string) (string, error) {
	if !routes.Known[service] {
		return "", Problems{}.UnknownService(service)
	}
	return service, nil
}

func (Routes) Of(services []string, prefix string) Routes {
	known := map[string]bool{}
	for _, service := range services {
		known[service] = true
	}
	return Routes{Known: known, Prefix: prefix}
}
