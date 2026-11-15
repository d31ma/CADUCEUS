import (
	"cmp"
	"slices"

	"mesail.dev/packages/core"
	"mesail.dev/packages/events"
	"mesail.dev/packages/inbound"
)

// @Service
type FailureService struct {
	// @Needs
	wire *WireService
}

func (service *FailureService) Known(failure error) error {
	errorTree := service.flattened(failure)
	first := service.picked(errorTree, slices.IndexFunc(errorTree, service.isKnown))
	return cmp.Or(service.coreAnswer(first), service.eventAnswer(first), first, service.unknown(failure))
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
	case *Unauthenticated, *Forbidden, *NotFound, *InvalidRequest, *Conflict, *Unavailable, *ChangedRecipients, *ChatNotAllowed,
		*TooLarge, *ChangedMessage, *core.EditConflict, *core.Forbidden, *core.Unavailable, *core.InvalidRequest, *core.NotFound, *core.Conflict, *core.RecipientsChanged,
		*core.ChatNotAllowed, *core.TooLarge, events.NotService, events.InvalidEnvelope, events.NotSubscribed, events.UnknownEventType,
		inbound.UnhandledEvent:
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

func (service *FailureService) coreAnswer(failure error) error {
	switch known := failure.(type) {
	case *core.InvalidRequest:
		return &InvalidRequest{Message: known.Message, Field: known.Field, ErrorNumber: known.Number()}
	case *core.Forbidden:
		return &Forbidden{Message: known.Message, ErrorNumber: known.Number()}
	case *core.NotFound:
		return &NotFound{Message: known.Message, ErrorNumber: known.Number()}
	case *core.Conflict:
		return &Conflict{Code: known.Code, Message: known.Message, ErrorNumber: known.Number()}
	case *core.EditConflict:
		return ChangedMessage{}.Of(known.Error(), service.wire.Message(known.Current), known.Number())
	case *core.RecipientsChanged:
		return ChangedRecipients{}.Of(known.Addresses, known.Number())
	case *core.ChatNotAllowed:
		return &ChatNotAllowed{Message: known.Error(), Address: known.Address, ErrorNumber: known.Number()}
	case *core.TooLarge:
		return &TooLarge{Message: known.Message, ErrorNumber: known.Number()}
	case *core.Unavailable:
		return &Unavailable{Message: known.Message, ErrorNumber: known.Number()}
	}
	return nil
}

func (service *FailureService) eventAnswer(failure error) error {
	switch known := failure.(type) {
	case events.InvalidEnvelope:
		return InvalidRequest{}.Kit(known.Error(), known.Number())
	case events.NotSubscribed:
		return InvalidRequest{}.Kit(known.Error(), known.Number())
	case events.UnknownEventType:
		return InvalidRequest{}.Kit(known.Error(), known.Number())
	case inbound.UnhandledEvent:
		return InvalidRequest{}.UnhandledEvent(known.Error())
	case events.NotService:
		return Forbidden{}.Kit(known.Error(), known.Number())
	}
	return nil
}

func (service *FailureService) unknown(failure error) error {
	if failure != nil {
		return Unavailable{}.ServiceFailed()
	}
	return nil
}
