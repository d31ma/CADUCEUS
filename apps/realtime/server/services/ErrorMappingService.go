import (
	"cmp"
	"errors"

	"mesail.dev/packages/events"
	"mesail.dev/packages/realtime"
)

// @Service
type ErrorMappingService struct {
}

func (service *ErrorMappingService) EventError(failed error) error {
	return service.answer(failed, []func(error) error{service.envelope, service.eventType, service.invalid, service.unauthenticated, service.notService, service.forbidden, service.missing})
}

func (service *ErrorMappingService) GatewayError(failed error) error {
	return service.answer(failed, []func(error) error{service.invalid, service.missing, service.unauthenticated})
}

func (service *ErrorMappingService) SweepError(failed error) error {
	return service.answer(failed, []func(error) error{service.missing, service.forbidden, service.unauthenticated})
}

func (service *ErrorMappingService) PresenceError(failed error) error {
	return service.answer(failed, []func(error) error{service.invalid, service.unauthenticated, service.missing})
}

func (service *ErrorMappingService) TicketError(failed error) error {
	return service.answer(failed, []func(error) error{service.unauthenticated})
}

func (service *ErrorMappingService) answer(failed error, rules []func(error) error) error {
	return cmp.Or(service.answers(failed, rules)...)
}

func (service *ErrorMappingService) answers(failed error, rules []func(error) error) []error {
	answers := []error{}
	for _, rule := range append(rules, service.unexpected) {
		answers = append(answers, rule(failed))
	}
	return answers
}

func (service *ErrorMappingService) unexpected(failed error) error {
	if failed == nil {
		return nil
	}
	return Unavailable{}.RouteFailed()
}

func (service *ErrorMappingService) envelope(failed error) error {
	refused := events.InvalidEnvelope{}
	if !errors.As(failed, &refused) {
		return nil
	}
	return InvalidRequest{}.EnvelopeRefused(refused.Reason)
}

func (service *ErrorMappingService) eventType(failed error) error {
	if !errors.As(failed, &events.NotSubscribed{}) && !errors.As(failed, &events.UnknownEventType{}) {
		return nil
	}
	return InvalidRequest{}.TypeRefused(failed.Error())
}

func (service *ErrorMappingService) invalid(failed error) error {
	invalid := realtime.Invalid{}
	if !errors.As(failed, &invalid) {
		return nil
	}
	return &InvalidRequest{Message: invalid.Reason, Field: invalid.Field, ErrorNumber: invalid.Number()}
}

func (service *ErrorMappingService) notService(failed error) error {
	if !errors.Is(failed, events.NotService{}) {
		return nil
	}
	return Forbidden{}.EventCallerNotAService()
}

func (service *ErrorMappingService) unauthenticated(failed error) error {
	refused := realtime.Unauthenticated{}
	if !errors.As(failed, &refused) {
		return nil
	}
	return &Unauthenticated{ErrorNumber: refused.Number()}
}

func (service *ErrorMappingService) forbidden(failed error) error {
	refused := realtime.Forbidden{}
	if !errors.As(failed, &refused) {
		return nil
	}
	return &Forbidden{ErrorNumber: refused.Number()}
}

func (service *ErrorMappingService) missing(failed error) error {
	missing := realtime.Missing{}
	if !errors.As(failed, &missing) {
		return nil
	}
	return &NotFound{ErrorNumber: missing.Number()}
}
