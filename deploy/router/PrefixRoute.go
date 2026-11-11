package main

import "net/http"

type PrefixRoute struct {
	Prefix  string
	Handler http.Handler
}
