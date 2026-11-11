package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Program struct {
}

func (program Program) Run() int {
	return program.exitCode(program.run())
}

func (program Program) run() error {
	settings, err := SettingsReader{Lookup: os.LookupEnv}.Read()
	if err != nil {
		return err
	}
	server := &http.Server{Addr: settings.Listen, Handler: Assembly{}.Handler(settings), ReadHeaderTimeout: program.headerTimeout()}
	signals, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()
	failures := make(chan error, 1)
	go program.listen(server, failures)
	slog.Info("router listening", "address", settings.Listen, "services", len(settings.Services))
	return program.wait(signals, server, failures)
}

func (program Program) listen(server *http.Server, failures chan<- error) {
	failures <- program.unlessClosed(server.ListenAndServe())
}

func (Program) unlessClosed(err error) error {
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (program Program) wait(signals context.Context, server *http.Server, failures <-chan error) error {
	select {
	case err := <-failures:
		return err
	case <-signals.Done():
		return program.shutdown(server)
	}
}

func (program Program) shutdown(server *http.Server) error {
	deadline, cancel := context.WithTimeout(context.Background(), program.shutdownTimeout())
	defer cancel()
	slog.Info("router stopping")
	return server.Shutdown(deadline)
}

func (Program) exitCode(err error) int {
	if err != nil {
		slog.Error("router failed", "error", err.Error())
		return 1
	}
	return 0
}

func (Program) headerTimeout() time.Duration {
	return 10 * time.Second
}

func (Program) shutdownTimeout() time.Duration {
	return 20 * time.Second
}
