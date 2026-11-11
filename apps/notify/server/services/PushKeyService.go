import (
	"cmp"

	"mesail.dev/packages/webpush"
)

// @Service
type PushKeyService struct {
	// @Needs
	callers *CallerService
	// @Needs
	settings *NotifySettingsService
}

func (service *PushKeyService) Key(authorization string) (PushKey, error) {
	_, failure := service.callers.Caller(authorization)
	keys, problem := webpush.VapidSettingsReader{}.FromApplicationJSON(service.settings.Snapshot())
	return PushKey{PublicKey: service.publicKey(cmp.Or(failure, problem), keys)}, cmp.Or(failure, service.notConfigured(problem))
}

func (service *PushKeyService) publicKey(failure error, keys webpush.VapidKeys) string {
	if failure != nil {
		return ""
	}
	return keys.PublicKey
}

func (service *PushKeyService) notConfigured(problem error) error {
	if problem != nil {
		return &PushNotConfigured{}
	}
	return nil
}
