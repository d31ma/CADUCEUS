package mailin

import (
	"encoding/json"

	"mesail.dev/packages/outbox"
)

type SettingsReader struct {
	Environment func(string) string
}

func (reader SettingsReader) Outbox() (outbox.Settings, error) {
	snapshot := ApplicationSnapshot{}
	err := json.Unmarshal([]byte(reader.Environment(reader.applicationVariable())), &snapshot)
	if err != nil {
		return outbox.Settings{}, reader.unavailable()
	}
	settings, err := outbox.SettingsReader{}.FromApplicationJSON(string(snapshot.Yon))
	if err != nil {
		return outbox.Settings{}, reader.unavailable()
	}
	return settings, nil
}

func (reader SettingsReader) ServiceToken() outbox.TokenSource {
	return outbox.StaticToken{Value: reader.Environment(reader.tokenVariable())}
}

func (SettingsReader) unavailable() error {
	return Unavailable{Operation: "outbox settings"}
}

func (SettingsReader) applicationVariable() string {
	return "YON_APPLICATION_JSON"
}

func (SettingsReader) tokenVariable() string {
	return "YON_SERVICE_TOKEN"
}
