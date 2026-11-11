import (
	"encoding/json"
	"os"

	"mesail.dev/packages/outbox"
)

// @Service
type NotifySettingsService struct{}

func (settings *NotifySettingsService) Snapshot() string {
	return os.Getenv(settings.variable())
}

func (settings *NotifySettingsService) Outbox() (outbox.Settings, error) {
	found, failure := outbox.SettingsReader{}.FromApplicationJSON(settings.application())
	return found, settings.unavailable(failure)
}

func (settings *NotifySettingsService) Options() NotifyOptions {
	application := ApplicationSettings{}
	_ = json.Unmarshal([]byte(settings.application()), &application)
	return application.Notify
}

func (settings *NotifySettingsService) application() string {
	snapshot := ApplicationSnapshot{}
	_ = json.Unmarshal([]byte(settings.Snapshot()), &snapshot)
	return settings.orEmpty(snapshot.Yon)
}

func (settings *NotifySettingsService) orEmpty(application json.RawMessage) string {
	if len(application) == 0 {
		return "{}"
	}
	return string(application)
}

func (settings *NotifySettingsService) unavailable(failure error) error {
	if failure != nil {
		return &StorageUnavailable{Operation: "read the outbox settings"}
	}
	return nil
}

func (settings *NotifySettingsService) variable() string {
	return "YON_APPLICATION_JSON"
}
