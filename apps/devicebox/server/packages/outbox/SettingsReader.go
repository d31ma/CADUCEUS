package outbox

import (
	"encoding/json"
	"os"
)

type SettingsReader struct{}

func (reader SettingsReader) FromEnvironment() (Settings, error) {
	return reader.FromApplicationJSON(os.Getenv(reader.variable()))
}

func (reader SettingsReader) FromApplicationJSON(raw string) (Settings, error) {
	file := ApplicationFile{}
	err := json.Unmarshal([]byte(raw), &file)
	if err != nil {
		return Settings{}, err
	}
	settings, err := reader.present(file.outbox())
	if err != nil {
		return Settings{}, err
	}
	settings.Retry = settings.Retry.withDefaults()
	return settings, nil
}

func (SettingsReader) present(outbox *Settings) (Settings, error) {
	if outbox == nil {
		return Settings{}, MissingSettings{}
	}
	return *outbox, nil
}

func (SettingsReader) variable() string {
	return "YON_APPLICATION_JSON"
}
