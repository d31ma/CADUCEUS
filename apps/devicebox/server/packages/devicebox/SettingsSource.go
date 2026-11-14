package devicebox

import (
	"os"

	"mesail.dev/packages/outbox"
)

type SettingsSource struct{}

func (source SettingsSource) FromEnvironment() Settings {
	return source.FromSnapshot(os.Getenv("YON_APPLICATION_JSON"))
}

func (SettingsSource) FromSnapshot(snapshot string) Settings {
	box, err := outbox.SettingsReader{}.FromApplicationJSON(snapshot)
	return Settings{Outbox: box, OutboxError: err}
}
