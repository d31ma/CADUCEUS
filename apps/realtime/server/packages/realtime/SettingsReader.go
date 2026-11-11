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
	gateway := snapshot.Yon.Realtime.Gateway
	return Settings{GatewayUrl: strings.TrimSuffix(gateway.Url, "/"), GatewaySecret: gateway.Secret}
}

func (SettingsReader) variable() string {
	return "YON_APPLICATION_JSON"
}
