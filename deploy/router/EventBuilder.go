package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"net"
	"net/http"
	"strings"
	"time"
)

type EventBuilder struct {
	Now func() time.Time
}

func (builder EventBuilder) Build(request *http.Request, body []byte) HttpEvent {
	now := builder.Now()
	host, _, _ := strings.Cut(request.Host, ":")
	return HttpEvent{
		Version:               "2.0",
		RouteKey:              builder.routeKey(),
		RawPath:               request.URL.EscapedPath(),
		RawQueryString:        request.URL.RawQuery,
		Cookies:               builder.cookies(request.Header.Get("Cookie")),
		Headers:               builder.headers(request),
		QueryStringParameters: builder.query(request),
		RequestContext: EventContext{
			AccountId:    "000000000000",
			ApiId:        "mesail-router",
			DomainName:   request.Host,
			DomainPrefix: strings.Split(host, ".")[0],
			Http:         builder.method(request),
			RequestId:    builder.requestId(),
			RouteKey:     builder.routeKey(),
			Stage:        builder.routeKey(),
			Time:         now.UTC().Format("02/Jan/2006:15:04:05 -0700"),
			TimeEpoch:    now.UnixMilli(),
		},
		Body:            base64.StdEncoding.EncodeToString(body),
		IsBase64Encoded: true,
	}
}

func (EventBuilder) routeKey() string {
	return "$default"
}

func (builder EventBuilder) method(request *http.Request) EventMethod {
	return EventMethod{
		Method:    request.Method,
		Path:      request.URL.Path,
		Protocol:  request.Proto,
		SourceIp:  builder.sourceIp(request.RemoteAddr),
		UserAgent: request.UserAgent(),
	}
}

func (EventBuilder) sourceIp(address string) string {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return address
	}
	return host
}

func (EventBuilder) cookies(header string) []string {
	if header == "" {
		return nil
	}
	return strings.Split(header, "; ")
}

func (builder EventBuilder) headers(request *http.Request) map[string]string {
	copied := request.Header.Clone()
	builder.dropHopByHop(copied)
	headers := builder.joined(copied)
	headers["host"] = "localhost"
	headers["x-forwarded-host"] = request.Host
	headers["x-forwarded-proto"] = builder.scheme(request)
	headers["x-forwarded-for"] = builder.sourceIp(request.RemoteAddr)
	return headers
}

func (EventBuilder) dropHopByHop(header http.Header) {
	for _, name := range []string{"Connection", "Keep-Alive", "Proxy-Connection", "Te", "Trailer", "Transfer-Encoding", "Upgrade", "Cookie", "Host"} {
		header.Del(name)
	}
}

func (EventBuilder) joined(header http.Header) map[string]string {
	joined := map[string]string{}
	for name, values := range header {
		joined[strings.ToLower(name)] = strings.Join(values, ",")
	}
	return joined
}

func (EventBuilder) query(request *http.Request) map[string]string {
	joined := map[string]string{}
	for name, values := range request.URL.Query() {
		joined[name] = strings.Join(values, ",")
	}
	return joined
}

func (EventBuilder) scheme(request *http.Request) string {
	if request.TLS != nil {
		return "https"
	}
	return "http"
}

func (EventBuilder) requestId() string {
	random := make([]byte, 16)
	_, _ = rand.Read(random)
	return hex.EncodeToString(random)
}
