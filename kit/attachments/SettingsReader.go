package attachments

import (
	"cmp"
	"encoding/json"
)

type SettingsReader struct {
	Environment func(string) string
}

func (reader SettingsReader) Read() Settings {
	snapshot := Snapshot{}
	json.Unmarshal([]byte(reader.Environment("YON_APPLICATION_JSON")), &snapshot)
	return reader.withDefaults(snapshot.Yon.Attachments)
}

func (reader SettingsReader) withDefaults(settings Settings) Settings {
	settings.Adapter = cmp.Or(settings.Adapter, reader.localAdapter())
	settings.Link.Seconds = cmp.Or(settings.Link.Seconds, reader.defaultLinkSeconds())
	settings.Limit.Bytes = cmp.Or(settings.Limit.Bytes, reader.defaultLimitBytes())
	return settings
}

func (SettingsReader) localAdapter() string {
	return "local"
}

func (SettingsReader) defaultLinkSeconds() int64 {
	return 300
}

func (SettingsReader) defaultLimitBytes() int64 {
	return 26214400
}
