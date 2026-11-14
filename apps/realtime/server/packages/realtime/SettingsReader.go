package realtime

import (
	"encoding/json"
	"strings"
)

type SettingsReader struct {
	Environment func(string) string
}

func (reader SettingsReader) Read() Settings {
	snapshot := SettingsSnapshot{}
	err := json.Unmarshal([]byte(reader.Environment(reader.variable())), &snapshot)
	if err != nil {
		return Settings{}
	}
	realtime := snapshot.Yon.Realtime
	return Settings{GatewayUrl: strings.TrimSuffix(realtime.Gateway.Url, "/"), GatewaySecret: realtime.Gateway.Secret, DirectoryUrl: realtime.DirectoryUrl, NotifyUrl: realtime.NotifyUrl, CallKeys: realtime.Calls}
}

func (SettingsReader) variable() string {
	return "YON_APPLICATION_JSON"
}
