package main

import (
	"maps"
	"net/http"
)

type SecurityHeaders struct {
}

func (SecurityHeaders) Apply(reply Reply) Reply {
	header := http.Header{}
	maps.Copy(header, reply.Header)
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Referrer-Policy", "no-referrer")
	header.Set("X-Frame-Options", "DENY")
	header.Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'wasm-unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
	return Reply{Status: reply.Status, Header: header, Body: reply.Body}
}
