package realtime

import "testing"

func settingsFrom(document string) Settings {
	return SettingsReader{Environment: func(name string) string {
		if name != "YON_APPLICATION_JSON" {
			return ""
		}
		return document
	}}.Read()
}

func TestReadTakesTheGatewayUrlAndSecret(t *testing.T) {
	settings := settingsFrom(`{"yon":{"realtime":{"gateway":{"url":"http://127.0.0.1:8117/","secret":"s3cret"}}}}`)
	if settings.GatewayUrl != "http://127.0.0.1:8117" || settings.GatewaySecret != "s3cret" {
		t.Fatalf("got %+v", settings)
	}
}

func TestReadGivesEmptySettingsWhenTheyAreMissing(t *testing.T) {
	if settings := settingsFrom(`{"yon":{}}`); settings != (Settings{}) {
		t.Fatalf("got %+v", settings)
	}
}

func TestReadGivesEmptySettingsForBadJson(t *testing.T) {
	if settings := settingsFrom(`{"yon":{"realtime":{"gateway":{"url":5,"secret":"s3cret"}}}}`); settings != (Settings{}) {
		t.Fatalf("got %+v", settings)
	}
	if settings := settingsFrom(`{not json`); settings != (Settings{}) {
		t.Fatalf("got %+v", settings)
	}
}
