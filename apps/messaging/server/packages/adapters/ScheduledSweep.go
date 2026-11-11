package adapters

import (
	"encoding/json"
	"os"

	"mesail.dev/packages/oidc"
)

type ScheduledSweep struct{}

func (sweep ScheduledSweep) Run() error {
	trigger := NewRelayTrigger(oidc.ServiceTokensFactory{}.Configured(), JsonCall{}.DefaultTimeout())
	return trigger.Run(sweep.RelayUrl(os.Getenv(sweep.variable())))
}

func (ScheduledSweep) RelayUrl(applicationJson string) string {
	snapshot := RelaySnapshot{}
	_ = json.Unmarshal([]byte(applicationJson), &snapshot)
	return snapshot.Yon.Messaging.RelayUrl
}

func (ScheduledSweep) variable() string {
	return "YON_APPLICATION_JSON"
}
