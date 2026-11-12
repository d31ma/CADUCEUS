import (
	"cmp"
	"encoding/json"
	"os"
	"strings"

	"mesail.dev/packages/outbox"
	"mesail.dev/packages/webpush"
)

// @Service
type NotifySettingsService struct{}

func (settings *NotifySettingsService) snapshot() string {
	return os.Getenv(settings.variable())
}

func (settings *NotifySettingsService) Outbox() (outbox.Settings, error) {
	found, failure := outbox.SettingsReader{}.FromApplicationJSON(settings.snapshot())
	return found, settings.unavailable(failure)
}

func (settings *NotifySettingsService) Options() NotifyOptions {
	application := ApplicationSettings{}
	_ = json.Unmarshal([]byte(settings.application()), &application)
	return application.Notify
}

func (settings *NotifySettingsService) PushKeys() (webpush.VapidKeys, error) {
	keys, failure := webpush.VapidSettingsReader{}.FromApplicationJSON(settings.snapshot())
	return keys, cmp.Or(settings.pushTurnedOff(), failure)
}

func (settings *NotifySettingsService) pushTurnedOff() error {
	if settings.switchedOn(settings.Options().Push.Enabled) {
		return nil
	}
	return &PushTurnedOff{}
}

func (settings *NotifySettingsService) switchedOn(raw json.RawMessage) bool {
	return strings.EqualFold(strings.Trim(string(raw), `"`), "true")
}

func (settings *NotifySettingsService) application() string {
	document := ApplicationSnapshot{}
	_ = json.Unmarshal([]byte(settings.snapshot()), &document)
	return settings.orEmpty(document.Yon)
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
