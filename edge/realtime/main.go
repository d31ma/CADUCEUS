package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mesail.dev/edge/realtime/backend"
	"mesail.dev/edge/realtime/connections"
	"mesail.dev/edge/realtime/startup"
	"mesail.dev/edge/realtime/transport"
)

type Program struct{}

func (program Program) Run() error {
	settings, err := startup.NewSettingsReader(startup.NewEnvironment(os.LookupEnv)).Load()
	if err != nil {
		return err
	}
	events := connections.NewEventLog(os.Stderr)
	gateway := transport.NewGateway(program.gatewaySettings(settings), events)
	public := program.server(settings.PublicAddress, gateway.Public())
	management := program.server(settings.ManagementAddress, gateway.Management())
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	failures := make(chan error, 2)
	go program.serve(public, failures)
	go program.serve(management, failures)
	events.Started(settings.PublicAddress, settings.ManagementAddress)
	go gateway.Warm()
	err = program.waitForEnd(ctx, failures)
	events.Stopping()
	program.shutdown(gateway, public, management)
	return err
}

func (program Program) gatewaySettings(settings startup.Settings) transport.GatewaySettings {
	return transport.GatewaySettings{
		BackendUrl:     settings.BackendUrl,
		BackendTimeout: program.backendTimeout(),
		BackendRetry:   backend.DefaultRetryPolicy(),
		Secret:         settings.Secret,
		Origins:        settings.Origins,
		Session:        connections.DefaultSessionSettings(),
	}
}

func (program Program) server(address string, handler http.Handler) *http.Server {
	return &http.Server{Addr: address, Handler: handler, ReadHeaderTimeout: program.headerTimeout()}
}

func (Program) serve(server *http.Server, failures chan<- error) {
	failures <- server.ListenAndServe()
}

func (Program) waitForEnd(ctx context.Context, failures <-chan error) error {
	select {
	case <-ctx.Done():
		return nil
	case err := <-failures:
		return err
	}
}

func (program Program) shutdown(gateway transport.Gateway, servers ...*http.Server) {
	grace, cancel := context.WithTimeout(context.Background(), program.gracePeriod())
	defer cancel()
	_ = gateway.Close(grace)
	program.stopServers(grace, servers)
}

func (Program) stopServers(grace context.Context, servers []*http.Server) {
	for _, server := range servers {
		_ = server.Shutdown(grace)
	}
}

func (Program) backendTimeout() time.Duration { return 30 * time.Second }
func (Program) headerTimeout() time.Duration  { return 10 * time.Second }
func (Program) gracePeriod() time.Duration    { return 30 * time.Second }

func main() {
	err := (Program{}).Run()
	if err != nil {
		log.Fatal(err)
	}
}
