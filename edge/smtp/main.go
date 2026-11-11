package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mesail.dev/edge/smtp/protocol"
	"mesail.dev/edge/smtp/startup"
)

type Program struct{}

func (program Program) Run() error {
	logger := log.New(os.Stderr, "", log.LstdFlags|log.LUTC)
	settings, err := startup.NewSettingsReader(startup.NewEnvironment(os.LookupEnv)).Load()
	if err != nil {
		return err
	}
	serverSettings, err := startup.Wiring{}.Build(settings, logger)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", settings.Address)
	if err != nil {
		return err
	}
	server := protocol.NewServer(serverSettings)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	done := make(chan struct{})
	go program.shutdownOnSignal(ctx, server, done)
	logger.Printf("smtp listening on %s as %s", listener.Addr(), settings.Hostname)
	serveError := server.Serve(listener)
	stop()
	<-done
	return serveError
}

func (program Program) shutdownOnSignal(ctx context.Context, server *protocol.Server, done chan struct{}) {
	<-ctx.Done()
	grace, cancel := context.WithTimeout(context.Background(), program.gracePeriod())
	defer cancel()
	server.Shutdown(grace)
	close(done)
}

func (Program) gracePeriod() time.Duration {
	return 30 * time.Second
}

func main() {
	if err := (Program{}).Run(); err != nil {
		log.Fatal(err)
	}
}
