import (
	"cmp"
	"errors"

	"mesail.dev/packages/oidc"
	"mesail.dev/packages/webpush"
)

// @Service
type PushSubscriptionsService struct {
	// @Needs
	callers *CallerService
	// @Needs
	store *PushSubscriptionStoreService
	// @Needs
	settings *NotifySettingsService
	// @Needs
	timestamps *TimestampService
}

func (service *PushSubscriptionsService) Subscribe(authorization string, request PushSubscriptionRequest) error {
	caller, failure := service.callers.Caller(authorization)
	failure = cmp.Or(failure, service.checkEndpoint(request.Endpoint), service.checkKeys(request))
	return service.saveUnless(failure, service.record(caller, request))
}

func (service *PushSubscriptionsService) Unsubscribe(authorization string, request PushSubscriptionRequest) error {
	caller, failure := service.callers.Caller(authorization)
	return service.deleteUnless(failure, caller, request.Endpoint)
}

func (service *PushSubscriptionsService) checkEndpoint(endpoint string) error {
	refusal := webpush.EndpointPolicy{Hosts: service.settings.Options().Push.Hosts}.Check(endpoint)
	return service.invalid(refusal, "endpoint")
}

func (service *PushSubscriptionsService) checkKeys(request PushSubscriptionRequest) error {
	if request.PublicKey == "" || request.Auth == "" || len(request.PublicKey) > service.keyLimit() || len(request.Auth) > service.keyLimit() {
		return &InvalidRequest{Field: "p256dh", Reason: "the subscription keys p256dh and auth are required"}
	}
	return nil
}

func (service *PushSubscriptionsService) invalid(refusal error, field string) error {
	refused := webpush.RefusedEndpoint{}
	if errors.As(refusal, &refused) {
		return &InvalidRequest{Field: field, Reason: refused.Reason}
	}
	return refusal
}

func (service *PushSubscriptionsService) record(caller oidc.Caller, request PushSubscriptionRequest) PushSubscriptionRecord {
	return PushSubscriptionRecord{
		DomainId:  caller.DomainId,
		MemberId:  caller.MemberId,
		Platform:  webpush.Platforms{}.Web(),
		Endpoint:  request.Endpoint,
		P256dh:    request.PublicKey,
		Auth:      request.Auth,
		CreatedAt: service.timestamps.Now(),
	}
}

func (service *PushSubscriptionsService) saveUnless(failure error, record PushSubscriptionRecord) error {
	if failure != nil {
		return failure
	}
	return service.store.Upsert(record)
}

func (service *PushSubscriptionsService) deleteUnless(failure error, caller oidc.Caller, endpoint string) error {
	if failure != nil {
		return failure
	}
	return service.store.DeleteForMember(caller.DomainId, caller.MemberId, endpoint)
}

func (service *PushSubscriptionsService) keyLimit() int {
	return 256
}
