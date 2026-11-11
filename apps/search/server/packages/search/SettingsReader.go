package search

import "encoding/json"

type SettingsReader struct {
	Environment func(string) string
}

func (reader SettingsReader) Read() Settings {
	snapshot := Snapshot{}
	json.Unmarshal([]byte(reader.Environment("YON_APPLICATION_JSON")), &snapshot)
	return snapshot.Yon.Search
}
