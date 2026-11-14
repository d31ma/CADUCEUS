import (
	"errors"

	"mesail.dev/packages/devicebox"
)

// @Service
type ErrorMappingService struct {
}

func (service *ErrorMappingService) Mailbox(failed error) error {
	return service.mapped(failed, Unavailable{}.MailboxRouteFailed())
}

func (service *ErrorMappingService) Outbox(failed error) error {
	return service.mapped(failed, Unavailable{}.OutboxRouteFailed())
}

func (service *ErrorMappingService) mapped(failed error, fallback *Unavailable) error {
	invalid := &devicebox.InvalidRequest{}
	unauthenticated := &devicebox.Unauthenticated{}
	forbidden := &devicebox.Forbidden{}
	missing := &devicebox.NotFound{}
	duplicate := &devicebox.Duplicate{}
	tooLarge := &devicebox.TooLarge{}
	full := &devicebox.BoxFull{}
	unavailable := &devicebox.Unavailable{}
	switch {
	case failed == nil:
		return nil
	case errors.As(failed, &invalid):
		return &InvalidRequest{Message: invalid.Message, Field: invalid.Field, ErrorNumber: invalid.Number()}
	case errors.As(failed, &unauthenticated):
		return &Unauthenticated{Message: unauthenticated.Message, ErrorNumber: unauthenticated.Number()}
	case errors.As(failed, &forbidden):
		return &Forbidden{Message: forbidden.Message, ErrorNumber: forbidden.Number()}
	case errors.As(failed, &missing):
		return &NotFound{Message: missing.Message, ErrorNumber: missing.Number()}
	case errors.As(failed, &duplicate):
		return &Duplicate{Message: duplicate.Message, ErrorNumber: duplicate.Number()}
	case errors.As(failed, &tooLarge):
		return &TooLarge{Message: tooLarge.Message, ErrorNumber: tooLarge.Number()}
	case errors.As(failed, &full):
		return &BoxFull{Message: full.Message, ErrorNumber: full.Number()}
	case errors.As(failed, &unavailable):
		return &Unavailable{Message: unavailable.Message, ErrorNumber: unavailable.Number()}
	}
	return fallback
}
