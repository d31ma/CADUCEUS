import (
	"cmp"
	"slices"

	"mesail.dev/packages/core"
	"mesail.dev/packages/events"
	"mesail.dev/packages/inbound"
)

// @Service
type FailureService struct {
}

func (service *FailureService) Known(failure error) error {
	errorTree := service.flattened(failure)
	first := service.picked(errorTree, slices.IndexFunc(errorTree, service.isKnown))
	return cmp.Or(service.withoutCause(first), service.unknown(failure))
}

func (service *FailureService) flattened(failure error) []error {
	found := []error{}
	waiting := []error{failure}
	for len(waiting) > 0 {
		found = append(found, waiting[0])
		waiting = append(waiting[1:], service.children(waiting[0])...)
	}
	return found
}

func (service *FailureService) children(failure error) []error {
	switch wrapper := failure.(type) {
	case interface{ Unwrap() []error }:
		return wrapper.Unwrap()
	case interface{ Unwrap() error }:
		return []error{wrapper.Unwrap()}
	}
	return []error{}
}

func (service *FailureService) isKnown(failure error) bool {
	switch failure.(type) {
	case *Unauthenticated, *core.Forbidden, *core.Unavailable, *core.InvalidRequest, *core.NotFound,
		*core.Conflict, *core.RecipientsChanged, *core.ChatNotAllowed, events.NotService,
		events.InvalidEnvelope, events.NotSubscribed, events.UnknownEventType, inbound.UnhandledEvent:
		return true
	}
	return false
}

func (service *FailureService) picked(errorTree []error, index int) error {
	if index < 0 {
		return nil
	}
	return errorTree[index]
}

func (service *FailureService) withoutCause(failure error) error {
	unavailable, isUnavailable := failure.(*core.Unavailable)
	if isUnavailable {
		return &core.Unavailable{Message: unavailable.Message}
	}
	return failure
}

func (service *FailureService) unknown(failure error) error {
	if failure != nil {
		return &core.Unavailable{Message: "the messaging service is not available", Cause: failure}
	}
	return nil
}
