package adapters

import (
	"net/http"
	"strconv"
	"time"

	"mesail.dev/packages/core"
)

type RelayTrigger struct {
	tokens ServiceAuthorizer
	client *http.Client
}

func NewRelayTrigger(tokens ServiceAuthorizer, timeout time.Duration) RelayTrigger {
	return RelayTrigger{tokens: tokens, client: &http.Client{Transport: http.DefaultTransport, Timeout: timeout}}
}

func (trigger RelayTrigger) Run(relayUrl string) error {
	authorization, err := trigger.tokens.Authorization(trigger.operatorRealm())
	if err != nil {
		return err
	}
	request, err := http.NewRequest(http.MethodPost, relayUrl, http.NoBody)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", authorization)
	response, err := trigger.client.Do(request)
	if err != nil {
		return err
	}
	response.Body.Close()
	return trigger.answer(response.StatusCode)
}

func (trigger RelayTrigger) answer(status int) error {
	if status/trigger.classSize() == trigger.successClass() {
		return nil
	}
	return &core.Unavailable{Message: "the outbox relay route answered " + strconv.Itoa(status)}
}

func (RelayTrigger) operatorRealm() string {
	return "mesail-operators"
}

func (RelayTrigger) classSize() int {
	return 100
}

func (RelayTrigger) successClass() int {
	return 2
}
