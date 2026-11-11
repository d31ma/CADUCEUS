import (
	"encoding/json"
	"os"

	"mesail.dev/packages/core"
	"mesail.dev/packages/outbox"
)

// @Service
type SettingsService struct {
}

func (service *SettingsService) Settings() MessagingSettings {
	application := ApplicationSettings{}
	json.Unmarshal([]byte(service.application()), &application)
	return application.Messaging
}

func (service *SettingsService) Outbox() (outbox.Settings, error) {
	settings, failure := outbox.SettingsReader{}.FromApplicationJSON(service.application())
	return settings, service.unavailable(failure)
}

func (service *SettingsService) application() string {
	snapshot := SettingsSnapshot{}
	json.Unmarshal([]byte(os.Getenv(service.variable())), &snapshot)
	return service.orEmpty(snapshot.Yon)
}

func (service *SettingsService) orEmpty(application json.RawMessage) string {
	if len(application) == 0 {
		return "{}"
	}
	return string(application)
}

func (service *SettingsService) unavailable(failure error) error {
	if failure == nil {
		return nil
	}
	return &core.Unavailable{Message: "the outbox settings are missing", Cause: failure}
}

func (service *SettingsService) variable() string {
	return "YON_APPLICATION_JSON"
}
