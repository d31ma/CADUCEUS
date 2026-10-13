// Package httpserver implements CADUCEUS production HTTP and operations listeners.
package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/d31ma/caduceus/internal/platform/buildinfo"
	"github.com/d31ma/caduceus/internal/platform/config"
	"github.com/d31ma/caduceus/internal/platform/preflight"
	"github.com/d31ma/caduceus/internal/system"
)

const (
	readHeaderTimeout = 5 * time.Second
	idleTimeout       = 75 * time.Second
)

// Runtime owns both listeners and their shared readiness lifecycle.
type Runtime struct {
	cfg     config.Config
	report  preflight.Report
	logger  *slog.Logger
	state   *system.Service
	metrics *Metrics
	public  *http.Server
	ops     *http.Server
}

// New constructs a ready-to-bind Phase 1 HTTP runtime.
func New(cfg config.Config, report preflight.Report, info buildinfo.Info, logger *slog.Logger) *Runtime {
	state := system.New(info, time.Now())
	metrics := newMetrics()
	runtime := &Runtime{cfg: cfg, report: report, logger: logger, state: state, metrics: metrics}
	runtime.public = newHTTPServer(cfg.HTTPAddress(), instrument(logger, metrics, runtime.publicHandler()))
	runtime.ops = newHTTPServer(cfg.MetricsAddress(), instrument(logger, metrics, runtime.operationsHandler()))
	return runtime
}

func newHTTPServer(address string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    1 << 20,
	}
}

// Run binds both sockets before serving and performs bounded graceful shutdown.
func (runtime *Runtime) Run(ctx context.Context) error {
	publicListener, err := net.Listen("tcp", runtime.public.Addr)
	if err != nil {
		return fmt.Errorf("bind HTTP listener: %w", err)
	}
	operationsListener, err := net.Listen("tcp", runtime.ops.Addr)
	if err != nil {
		_ = publicListener.Close()
		return fmt.Errorf("bind metrics listener: %w", err)
	}

	errorsFound := make(chan error, 2)
	go serve(runtime.public, publicListener, "HTTP", errorsFound)
	go serve(runtime.ops, operationsListener, "metrics", errorsFound)
	runtime.state.MarkReady(true)
	runtime.logger.Info(
		"server started",
		"event", "server_started",
		"http_address", runtime.public.Addr,
		"metrics_address", runtime.ops.Addr,
		"fylo_version", runtime.report.FYLOVersion,
	)

	var serveErr error
	select {
	case <-ctx.Done():
	case serveErr = <-errorsFound:
	}

	runtime.state.MarkReady(false)
	runtime.logger.Info("server shutdown started", "event", "shutdown_started")
	shutdownContext, cancel := context.WithTimeout(context.Background(), runtime.cfg.ShutdownDuration())
	defer cancel()
	if err := runtime.shutdown(shutdownContext); err != nil && serveErr == nil {
		serveErr = err
	}
	runtime.logger.Info("server stopped", "event", "server_stopped")
	return serveErr
}

func serve(server *http.Server, listener net.Listener, name string, result chan<- error) {
	err := server.Serve(listener)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		result <- fmt.Errorf("%s server: %w", name, err)
	}
}

func (runtime *Runtime) shutdown(ctx context.Context) error {
	var group sync.WaitGroup
	var mu sync.Mutex
	var failures []error
	for _, server := range []*http.Server{runtime.public, runtime.ops} {
		group.Add(1)
		go func(target *http.Server) {
			defer group.Done()
			if err := target.Shutdown(ctx); err != nil {
				mu.Lock()
				failures = append(failures, err)
				mu.Unlock()
			}
		}(server)
	}
	group.Wait()
	return errors.Join(failures...)
}

func (runtime *Runtime) publicHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", runtime.health)
	mux.HandleFunc("GET /healthz", runtime.health)
	mux.HandleFunc("GET /readyz", runtime.ready)
	mux.HandleFunc("GET /system", runtime.system)
	mux.HandleFunc("GET /api/v1/system", runtime.system)
	mux.HandleFunc("/api/", func(response http.ResponseWriter, request *http.Request) {
		problem(response, request, http.StatusNotFound, "resource_not_found", "API route not found")
	})
	mux.Handle("/", staticHandler(runtime.cfg.StaticDir, runtime.report.FrontendAssets))
	return mux
}

func (runtime *Runtime) operationsHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", runtime.health)
	mux.HandleFunc("GET /readyz", runtime.ready)
	mux.HandleFunc("GET /metrics", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		runtime.metrics.writePrometheus(response, runtime.state.Ready())
	})
	return mux
}

func (runtime *Runtime) health(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]any{"status": "ok"})
}

func (runtime *Runtime) ready(response http.ResponseWriter, _ *http.Request) {
	if !runtime.state.Ready() {
		writeJSON(response, http.StatusServiceUnavailable, map[string]any{"status": "not_ready"})
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"status": "ready"})
}

func (runtime *Runtime) system(response http.ResponseWriter, request *http.Request) {
	document := struct {
		system.Snapshot
		RequestID string `json:"requestId"`
	}{
		Snapshot:  runtime.state.Snapshot(time.Now()),
		RequestID: requestID(request.Context()),
	}
	writeJSON(response, http.StatusOK, document)
}

func problem(response http.ResponseWriter, request *http.Request, status int, code, detail string) {
	response.Header().Set("Content-Type", "application/problem+json")
	writeJSON(response, status, map[string]any{
		"type":      "https://caduceus.del.ma/problems/" + code,
		"title":     http.StatusText(status),
		"status":    status,
		"code":      code,
		"detail":    detail,
		"instance":  request.URL.Path,
		"requestId": requestID(request.Context()),
	})
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	if response.Header().Get("Content-Type") == "" {
		response.Header().Set("Content-Type", "application/json; charset=utf-8")
	}
	response.Header().Set("Cache-Control", "no-store")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}
