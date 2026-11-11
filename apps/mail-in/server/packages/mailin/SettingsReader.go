package mailin

import "mesail.dev/packages/outbox"

type SettingsReader struct {
	Environment func(string) string
}

func (reader SettingsReader) Outbox() (outbox.Settings, error) {
	settings, err := outbox.SettingsReader{}.FromApplicationJSON(reader.Environment(reader.applicationVariable()))
	return settings, reader.unavailable(err)
}

func (reader SettingsReader) ServiceToken() outbox.TokenSource {
	return outbox.StaticToken{Value: reader.Environment(reader.tokenVariable())}
}

func (SettingsReader) unavailable(failure error) error {
	if failure == nil {
		return nil
	}
	return Unavailable{Operation: "outbox settings"}
}

func (SettingsReader) applicationVariable() string {
	return "YON_APPLICATION_JSON"
}

func (SettingsReader) tokenVariable() string {
	return "YON_SERVICE_TOKEN"
}
