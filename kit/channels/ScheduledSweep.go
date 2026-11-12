package channels

import (
	"net/http"
	"os"
	"strings"

	"mesail.dev/packages/oidc"
	"mesail.dev/packages/transport"
)

type ScheduledSweep struct {
	Tokens oidc.ServiceTokens
	Client *http.Client
}

func NewScheduledSweep() ScheduledSweep {
	return ScheduledSweep{Tokens: oidc.ServiceTokensFactory{}.Configured(), Client: transport.SingleUseTransport{}.Client(0)}
}

func (sweep ScheduledSweep) Run() error {
	return sweep.RunWith(os.Getenv("YON_APPLICATION_JSON"))
}

func (sweep ScheduledSweep) RunWith(snapshot string) error {
	authorization, err := sweep.Tokens.Authorization(sweep.installWide())
	if err != nil {
		return err
	}
	request, err := http.NewRequest(http.MethodPost, sweep.relayUrl(snapshot), strings.NewReader("{}"))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", authorization)
	request.Header.Set("Content-Type", "application/json")
	return sweep.perform(request)
}

func (sweep ScheduledSweep) perform(request *http.Request) error {
	response, err := sweep.Client.Do(request)
	if err != nil {
		return err
	}
	response.Body.Close()
	return sweep.answer(response.StatusCode)
}

func (ScheduledSweep) answer(status int) error {
	if status >= http.StatusOK && status < http.StatusMultipleChoices {
		return nil
	}
	return SweepRefused{Status: status}
}

func (ScheduledSweep) relayUrl(snapshot string) string {
	return SettingsSource{}.FromSnapshot(snapshot).RelayUrl
}

func (ScheduledSweep) installWide() string {
	return oidc.InstallWide{}.DomainId()
}
