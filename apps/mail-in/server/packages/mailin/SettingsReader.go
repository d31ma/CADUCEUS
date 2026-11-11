package mailin

import (
	"cmp"
	"encoding/json"

	"mesail.dev/packages/oidc"
	"mesail.dev/packages/outbox"
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
	return oidc.ServiceTokensFactory{}.FromJson(reader.Environment(reader.applicationVariable())).Authorization
}

func (reader SettingsReader) withDefaults(settings Settings) Settings {
	settings.Blobs.Adapter = cmp.Or(settings.Blobs.Adapter, reader.LocalAdapter())
	settings.Blobs.Bucket = cmp.Or(settings.Blobs.Bucket, reader.defaultBucket())
	settings.Blobs.Region = cmp.Or(reader.Environment("AWS_REGION"), settings.Blobs.Region, reader.defaultRegion())
	settings.Blobs.Endpoint = cmp.Or(reader.Environment("AWS_ENDPOINT_URL_S3"), reader.Environment("AWS_ENDPOINT_URL"), settings.Blobs.Endpoint)
	settings.Blobs.Link.Seconds = cmp.Or(settings.Blobs.Link.Seconds, reader.defaultLinkSeconds())
	return settings
}

func (SettingsReader) LocalAdapter() string {
	return "local"
}

func (SettingsReader) defaultBucket() string {
	return "mesail-raw"
}

func (SettingsReader) defaultRegion() string {
	return "us-east-1"
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
