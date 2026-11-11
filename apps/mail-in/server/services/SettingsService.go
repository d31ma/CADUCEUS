import (
	"encoding/json"
	"os"

	"mesail.dev/packages/outbox"
)

// @Service
type SettingsService struct {
}

func (service *SettingsService) Outbox() (outbox.Settings, error) {
	snapshot := ApplicationSnapshot{}
	err := json.Unmarshal([]byte(os.Getenv(service.variable())), &snapshot)
	if err != nil {
		return outbox.Settings{}, service.unavailable(err)
	}
	settings, err := outbox.SettingsReader{}.FromApplicationJSON(string(snapshot.Yon))
	return settings, service.unavailable(err)
}

func (service *SettingsService) ServiceToken() outbox.TokenSource {
	return outbox.StaticToken{Value: os.Getenv(service.tokenVariable())}
}

func (service *SettingsService) unavailable(failure error) error {
	if failure == nil {
		return nil
	}
	return &StorageUnavailable{Operation: "outbox settings"}
}

func (service *SettingsService) variable() string {
	return "YON_APPLICATION_JSON"
}

func (service *SettingsService) tokenVariable() string {
	return "YON_SERVICE_TOKEN"
}
