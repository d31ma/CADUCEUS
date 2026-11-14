package mailin

import (
	"cmp"
	"encoding/json"
	"time"

	"mesail.dev/packages/oidc"
	"mesail.dev/packages/outbox"
	"mesail.dev/packages/transport"
)

type SettingsReader struct {
	Environment func(string) string
}

func (reader SettingsReader) Outbox() (outbox.Settings, error) {
	settings, err := outbox.SettingsReader{}.FromApplicationJSON(reader.Environment(reader.applicationVariable()))
	return settings, reader.unavailable(err)
}

func (reader SettingsReader) MailIn() Settings {
	snapshot := ApplicationSnapshot{}
	json.Unmarshal([]byte(reader.Environment(reader.applicationVariable())), &snapshot)
	section := MailInSection{}
	json.Unmarshal(snapshot.Yon, &section)
	return reader.withDefaults(section.MailIn)
}

func (reader SettingsReader) Authorize() func(realm string) (string, error) {
	tokens := oidc.ServiceTokensFactory{}.FromJson(reader.Environment(reader.applicationVariable()))
	tokens.Client = transport.SingleUseTransport{}.Client(reader.tokenTimeout())
	return tokens.Authorization
}

func (reader SettingsReader) withDefaults(settings Settings) Settings {
	settings.Blobs.Adapter = cmp.Or(settings.Blobs.Adapter, reader.LocalAdapter())
	settings.Blobs.Link.Seconds = cmp.Or(settings.Blobs.Link.Seconds, reader.defaultLinkSeconds())
	return settings
}

func (SettingsReader) tokenTimeout() time.Duration {
	return 5 * time.Second
}

func (SettingsReader) LocalAdapter() string {
	return "local"
}

func (SettingsReader) defaultLinkSeconds() int64 {
	return 300
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
