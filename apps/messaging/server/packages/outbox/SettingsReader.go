package outbox

import "encoding/json"

type SettingsReader struct{}

func (reader SettingsReader) FromApplicationJSON(raw string) (Settings, error) {
	file := ApplicationFile{}
	err := json.Unmarshal([]byte(raw), &file)
	if err != nil {
		return Settings{}, err
	}
	settings, err := reader.present(file)
	if err != nil {
		return Settings{}, err
	}
	settings.Retry = settings.Retry.withDefaults()
	return settings, nil
}

func (SettingsReader) present(file ApplicationFile) (Settings, error) {
	if file.Outbox == nil {
		return Settings{}, MissingSettings{}
	}
	return *file.Outbox, nil
}
