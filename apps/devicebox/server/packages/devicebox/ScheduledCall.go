package devicebox

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"mesail.dev/packages/oidc"
	"mesail.dev/packages/transport"
)

type ScheduledCall struct {
	Path string
}

func (call ScheduledCall) Run(applicationJson string) error {
	authorization, err := oidc.ServiceTokensFactory{}.FromJson(applicationJson).Authorization(oidc.InstallWide{}.DomainId())
	if err != nil {
		return err
	}
	port, err := call.port(applicationJson)
	if err != nil {
		return err
	}
	request, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://127.0.0.1:%d%s", port, call.Path), nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", authorization)
	response, err := transport.SingleUseTransport{}.Client(call.timeout()).Do(request)
	if err != nil {
		return err
	}
	response.Body.Close()
	return call.outcome(response.StatusCode)
}

func (ScheduledCall) port(applicationJson string) (int, error) {
	snapshot := applicationSnapshot{}
	err := json.Unmarshal([]byte(applicationJson), &snapshot)
	if err != nil || snapshot.Yon.Server.Port == 0 {
		return 0, errors.Join(err, errors.New("the application settings name no server port"))
	}
	return snapshot.Yon.Server.Port, nil
}

func (ScheduledCall) outcome(status int) error {
	if status >= http.StatusMultipleChoices {
		return fmt.Errorf("the scheduled route answered %d", status)
	}
	return nil
}

func (ScheduledCall) timeout() time.Duration {
	return 60 * time.Second
}
