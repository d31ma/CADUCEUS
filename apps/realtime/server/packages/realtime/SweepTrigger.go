package realtime

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"mesail.dev/packages/oidc"
)

type SweepTrigger struct {
	Tokens      ServiceAuthorizer
	Environment func(string) string
	Client      *http.Client
}

func (trigger SweepTrigger) Configured(environment func(string) string) SweepTrigger {
	tokens := oidc.ServiceTokensFactory{}.FromJson(environment(trigger.settingsVariable()))
	return SweepTrigger{Tokens: tokens, Environment: environment, Client: &http.Client{Timeout: trigger.timeout()}}
}

func (trigger SweepTrigger) Fire() error {
	address, err := trigger.address()
	if err != nil {
		return err
	}
	authorization, err := trigger.Tokens.Authorization(trigger.operatorsRealm())
	if err != nil {
		return err
	}
	request, err := http.NewRequest(http.MethodPost, address, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", authorization)
	response, err := trigger.Client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	return trigger.answered(response.StatusCode)
}

func (trigger SweepTrigger) address() (string, error) {
	snapshot := ServerSnapshot{}
	err := json.Unmarshal([]byte(trigger.Environment(trigger.settingsVariable())), &snapshot)
	if err != nil || snapshot.Yon.Server.Port == 0 {
		return "", TriggerFailed{Reason: "yon.server.port is not set"}
	}
	return "http://127.0.0.1:" + strconv.Itoa(snapshot.Yon.Server.Port) + trigger.route(), nil
}

func (SweepTrigger) answered(status int) error {
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		return TriggerFailed{Reason: "the sweep route answered " + strconv.Itoa(status)}
	}
	return nil
}

func (SweepTrigger) settingsVariable() string {
	return "YON_APPLICATION_JSON"
}

func (SweepTrigger) operatorsRealm() string {
	return "mesail-operators"
}

func (SweepTrigger) route() string {
	return "/api/v1/realtime/jobs/sweep"
}

func (SweepTrigger) timeout() time.Duration {
	return 30 * time.Second
}
