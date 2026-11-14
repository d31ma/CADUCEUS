package devicebox

import (
	"encoding/json"
	"os"

	"mesail.dev/packages/outbox"
)

type SettingsSource struct{}

func (source SettingsSource) FromEnvironment() Settings {
	return source.FromSnapshot(os.Getenv("YON_APPLICATION_JSON"))
}

func (SettingsSource) FromSnapshot(snapshot string) Settings {
	document := settingsSnapshot{}
	_ = json.Unmarshal([]byte(snapshot), &document)
	box, err := outbox.SettingsReader{}.FromApplicationJSON(snapshot)
	return Settings{Outbox: box, OutboxError: err, DirectoryUrl: document.Yon.Devicebox.DirectoryUrl}
}
