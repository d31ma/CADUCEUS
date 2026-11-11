package channels

import (
	"encoding/json"
	"os"

	"mesail.dev/packages/outbox"
)

type SettingsSource struct{}

func (source SettingsSource) FromEnvironment() Settings {
	return source.FromSnapshot(os.Getenv(source.variable()))
}

func (SettingsSource) FromSnapshot(snapshot string) Settings {
	document := snapshotDocument{}
	_ = json.Unmarshal([]byte(snapshot), &document)
	box, err := outbox.SettingsReader{}.FromApplicationJSON(snapshot)
	return Settings{DirectoryUrl: document.Yon.Channels.DirectoryUrl, RelayUrl: document.Yon.Channels.RelayUrl, Outbox: box, OutboxError: err}
}

func (SettingsSource) variable() string {
	return "YON_APPLICATION_JSON"
}
