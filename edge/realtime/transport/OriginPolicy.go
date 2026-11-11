package transport

import (
	"net/http"
	"slices"

	"mesail.dev/edge/realtime/connections"
)

type OriginPolicy struct {
	allowed []string
	next    http.Handler
	log     connections.EventLog
}

func NewOriginPolicy(allowed []string, next http.Handler, log connections.EventLog) OriginPolicy {
	return OriginPolicy{allowed: allowed, next: next, log: log}
}

func (policy OriginPolicy) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	policy.admit(writer, request, request.Header.Get(Headers{}.Origin()))
}

func (policy OriginPolicy) admit(writer http.ResponseWriter, request *http.Request, origin string) {
	if !policy.allows(origin) {
		policy.log.OriginRefused()
		http.Error(writer, "origin not allowed", http.StatusForbidden)
		return
	}
	policy.next.ServeHTTP(writer, request)
}

func (policy OriginPolicy) allows(origin string) bool {
	return origin == "" || slices.Contains(policy.allowed, origin)
}
