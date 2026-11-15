package adapters

import (
	"net/url"
	"os"

	"mesail.dev/packages/oidc"
)

type ScheduledCleanup struct{}

func (cleanup ScheduledCleanup) Run() error {
	trigger := NewRelayTrigger(oidc.ServiceTokensFactory{}.Configured(), JsonCall{}.DefaultTimeout())
	return trigger.Run(cleanup.CleanupUrl(ScheduledSweep{}.RelayUrl(os.Getenv(cleanup.variable()))))
}

func (cleanup ScheduledCleanup) CleanupUrl(relayUrl string) string {
	parsed, err := url.Parse(relayUrl)
	if err != nil || relayUrl == "" {
		return ""
	}
	parsed.Path = cleanup.path()
	return parsed.String()
}

func (ScheduledCleanup) path() string {
	return "/api/v1/messaging/jobs/changes-cleanup"
}

func (ScheduledCleanup) variable() string {
	return "YON_APPLICATION_JSON"
}
