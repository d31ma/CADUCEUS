import (
	"encoding/json"
	"os"

	"mesail.dev/packages/outbox"
)

// @Service
type SettingsService struct {
}

func (service *SettingsService) Settings() DirectorySettings {
	settings := YonSettings{}
	json.Unmarshal(service.yon(), &settings)
	return settings.Directory
}

func (service *SettingsService) Outbox() (outbox.Settings, error) {
	return outbox.SettingsReader{}.FromEnvironment()
}

func (service *SettingsService) yon() []byte {
	snapshot := ApplicationSnapshot{}
	json.Unmarshal([]byte(os.Getenv(service.variable())), &snapshot)
	return snapshot.Yon
}

func (service *SettingsService) variable() string {
	return "YON_APPLICATION_JSON"
}
