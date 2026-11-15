package backups

import (
	"testing"
)

func TestSettingsReaderUsesDefaults(t *testing.T) {
	settings := SettingsReader{Environment: func(string) string { return "" }}.Read()
	if settings.Link.Seconds != 1800 || settings.Limit.Bytes != 268435456 {
		t.Fatalf("defaults %+v", settings)
	}
}

func TestSettingsReaderReadsTheBackupSection(t *testing.T) {
	text := `{"yon":{"attachments":{"backup":{"link":{"seconds":60},"limit":{"bytes":10}}}}}`
	settings := SettingsReader{Environment: func(string) string { return text }}.Read()
	if settings.Link.Seconds != 60 || settings.Limit.Bytes != 10 {
		t.Fatalf("settings %+v", settings)
	}
}

func TestDeskFactoryReportsAMissingLinkSecret(t *testing.T) {
	text := `{"yon":{"attachments":{"adapter":"local"}}}`
	factory := DeskFactory{Environment: func(name string) string {
		if name == "YON_APPLICATION_JSON" {
			return text
		}
		return ""
	}}
	fixture := newFixture(t)
	desk := factory.EventDesk(fixture.documents, fixture.documents.Scroll)
	if desk.Removal.Desk.Failure == nil || desk.Removal.Desk.Settings.Link.Seconds != 1800 {
		t.Fatalf("factory desk %+v", desk.Removal.Desk)
	}
}
