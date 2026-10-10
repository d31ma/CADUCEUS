package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type contextKey string

const requestIDKey contextKey = "request-id"

var validRequestID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{7,127}$`)

func instrument(logger *slog.Logger, metrics *Metrics, next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requestID := acceptedRequestID(request.Header.Get("X-Request-ID"))
		response.Header().Set("X-Request-ID", requestID)
		setSecurityHeaders(response.Header())

		started := time.Now()
		recorder := &statusRecorder{ResponseWriter: response, status: http.StatusOK}
		metrics.begin()
		next.ServeHTTP(recorder, request.WithContext(context.WithValue(request.Context(), requestIDKey, requestID)))
		duration := time.Since(started)
		route := normalizedRoute(request.URL.Path)
		metrics.end(route, request.Method, recorder.status, duration)
		logger.InfoContext(
			request.Context(),
			"http_request",
			"event", "http_request",
			"request_id", requestID,
			"method", request.Method,
			"route", route,
			"status", recorder.status,
			"duration_ms", duration.Milliseconds(),
			"remote_ip", remoteIP(request.RemoteAddr),
		)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (recorder *statusRecorder) WriteHeader(status int) {
	recorder.status = status
	recorder.ResponseWriter.WriteHeader(status)
}

func requestID(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey).(string)
	return value
}

func acceptedRequestID(candidate string) string {
	if validRequestID.MatchString(candidate) {
		return candidate
	}
	value := make([]byte, 16)
	if _, err := rand.Read(value); err == nil {
		return hex.EncodeToString(value)
	}
	return hex.EncodeToString([]byte(time.Now().UTC().Format("150405.000000")))
}

func setSecurityHeaders(header http.Header) {
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("X-Frame-Options", "DENY")
	header.Set("Referrer-Policy", "no-referrer")
	header.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
	header.Set(
		"Content-Security-Policy",
		"default-src 'self'; base-uri 'self'; object-src 'none'; frame-ancestors 'none'; "+
			"img-src 'self' data: blob:; style-src 'self' 'unsafe-inline'; script-src 'self'; "+
			"connect-src 'self' ws: wss:; frame-src 'self' data: blob:",
	)
}

func normalizedRoute(path string) string {
	switch path {
	case "/health", "/healthz":
		return "/healthz"
	case "/readyz":
		return "/readyz"
	case "/system", "/api/v1/system":
		return "/api/v1/system"
	case "/metrics":
		return "/metrics"
	default:
		if strings.HasPrefix(path, "/api/") {
			return "/api/*"
		}
		return "/static/*"
	}
}

func remoteIP(address string) string {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return ""
	}
	return host
}
