package backups

import (
	"cmp"
	"encoding/json"
)

type SettingsReader struct {
	Environment func(string) string
}

func (reader SettingsReader) Read() Settings {
	snapshot := SettingsSnapshot{}
	json.Unmarshal([]byte(reader.Environment("YON_APPLICATION_JSON")), &snapshot)
	return reader.withDefaults(snapshot.Yon.Attachments.Backup)
}

func (reader SettingsReader) withDefaults(settings Settings) Settings {
	settings.Link.Seconds = cmp.Or(settings.Link.Seconds, reader.defaultLinkSeconds())
	settings.Limit.Bytes = cmp.Or(settings.Limit.Bytes, reader.defaultLimitBytes())
	return settings
}

func (SettingsReader) defaultLinkSeconds() int64 {
	return 1800
}

func (SettingsReader) defaultLimitBytes() int64 {
	return 268435456
}
