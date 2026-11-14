package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mesail.dev/edge/calls/admission"
	"mesail.dev/edge/calls/desk"
	"mesail.dev/edge/calls/media"
	"mesail.dev/edge/calls/startup"
	"mesail.dev/edge/calls/transport"
	"mesail.dev/packages/oidc"
)

type Program struct{}

func (program Program) Run() error {
	settings, err := startup.NewSettingsReader(startup.NewEnvironment(os.LookupEnv)).Load()
	if err != nil {
		return err
	}
	connection, err := net.ListenPacket("udp", settings.MediaAddress)
	if err != nil {
		return err
	}
	defer connection.Close()
	events := media.NewEventLog(os.Stderr)
	calls := program.calls(settings, connection, events)
	server := &http.Server{Addr: settings.HttpAddress, Handler: transport.NewRoutes(program.desk(settings, calls)).Handler(), ReadHeaderTimeout: program.headerTimeout()}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	failures := make(chan error, 1)
	go program.serve(server, failures)
	events.Started(settings.HttpAddress, connection.LocalAddr().String())
	err = program.waitForEnd(ctx, failures)
	events.Stopping()
	calls.Close()
	program.shutdown(server)
	return err
}

func (program Program) calls(settings startup.Settings, connection net.PacketConn, events media.EventLog) *media.Calls {
	factory := media.NewPeerFactory(media.PeerOptions{
		PublicAddresses:     settings.PublicAddresses,
		DisconnectedTimeout: 5 * time.Second,
		FailedTimeout:       15 * time.Second,
		KeepAliveInterval:   2 * time.Second,
		StartBitrate:        1_000_000,
	}, connection)
	opener := media.NewMemberOpener(factory, admission.SystemClock{}, 20*time.Second)
	return media.NewCalls(opener, media.CallContext{Options: media.CallOptions{PlanInterval: 300 * time.Millisecond}, Clock: admission.SystemClock{}, Log: events})
}

func (program Program) desk(settings startup.Settings, calls *media.Calls) desk.CallDesk {
	fetcher := oidc.HttpKeyFetcher{Client: &http.Client{Timeout: program.headerTimeout()}}
	keys := admission.NewKeyBook(fetcher, settings.KeysUrl, admission.SystemClock{})
	return desk.NewCallDesk(admission.NewJoinTokenCheck(keys, admission.SystemClock{}), calls)
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

func (program Program) shutdown(server *http.Server) {
	grace, cancel := context.WithTimeout(context.Background(), program.gracePeriod())
	defer cancel()
	_ = server.Shutdown(grace)
}

func (Program) headerTimeout() time.Duration { return 10 * time.Second }
func (Program) gracePeriod() time.Duration   { return 10 * time.Second }

func main() {
	err := (Program{}).Run()
	if err != nil {
		log.Fatal(err)
	}
}
