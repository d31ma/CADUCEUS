package outbox

import (
	"errors"
	"testing"
)

func TestSettingsReaderReadsTheOutboxKeyWithDefaults(t *testing.T) {
	raw := `{"name":"channels","outbox":{"producer":"channels","subscribers":[{"name":"messaging","url":"http://127.0.0.1:8105/api/v1/messaging/events"}],"retry":{"maxAttempts":3}}}`

	settings, err := SettingsReader{}.FromApplicationJSON(raw)
	if err != nil || settings.Producer != "channels" || settings.Subscribers[0].Name != "messaging" {
		t.Fatalf("settings %+v, error %v", settings, err)
	}
	if settings.Retry != (RetrySettings{BaseDelaySeconds: 5, MaxDelaySeconds: 900, MaxAttempts: 3}) {
		t.Fatalf("retry %+v", settings.Retry)
	}
}

func TestSettingsReaderReportsAMissingOutboxKey(t *testing.T) {
	_, err := SettingsReader{}.FromApplicationJSON(`{"name":"channels"}`)
	if !errors.As(err, &MissingSettings{}) || err.Error() == "" {
		t.Fatalf("error %v", err)
	}
}

func TestSettingsReaderReportsBadJSON(t *testing.T) {
	_, err := SettingsReader{}.FromApplicationJSON(`{"outbox":`)
	if err == nil {
		t.Fatal("bad JSON should be reported")
	}
}
