import (
	"crypto/rand"
	"net/http"
	"time"

	"mesail.dev/packages/transport"
	"mesail.dev/packages/webpush"
)

// @Service
type PushTargetsService struct {
	// @Needs
	settings *NotifySettingsService
	client   *http.Client
}

func (service *PushTargetsService) ForPlatforms() (map[string]webpush.Target, error) {
	keys, failure := webpush.VapidSettingsReader{}.FromApplicationJSON(service.settings.Snapshot())
	policy := webpush.EndpointPolicy{Hosts: service.settings.Options().Push.Hosts}
	web := webpush.NewWebPushTarget(webpush.NewVapidSigner(keys, time.Now), webpush.NewEncryptor(rand.Reader), policy, service.clientOrDefault())
	targets := map[string]webpush.Target{webpush.Platforms{}.Web(): web}
	return service.configured(failure, targets), failure
}

func (service *PushTargetsService) configured(failure error, targets map[string]webpush.Target) map[string]webpush.Target {
	if failure != nil {
		return map[string]webpush.Target{}
	}
	return targets
}

func (service *PushTargetsService) clientOrDefault() *http.Client {
	if service.client != nil {
		return service.client
	}
	return transport.SingleUseTransport{}.Client(service.timeout())
}

func (service *PushTargetsService) timeout() time.Duration {
	return 10 * time.Second
}
