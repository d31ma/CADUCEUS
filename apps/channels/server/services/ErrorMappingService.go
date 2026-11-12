import (
	"errors"

	"mesail.dev/packages/channels"
)

// @Service
type ErrorMappingService struct {
}

func (service *ErrorMappingService) ChannelError(failed error) error {
	return service.channelError(failed)
}

func (service *ErrorMappingService) OutboxError(failed error) error {
	return service.outboxError(failed)
}

func (service *ErrorMappingService) channelError(failed error) error {
	invalid := &channels.InvalidRequest{}
	unauthenticated := &channels.Unauthenticated{}
	forbidden := &channels.Forbidden{}
	missing := &channels.NotFound{}
	conflict := &channels.Conflict{}
	duplicate := &channels.Duplicate{}
	unavailable := &channels.Unavailable{}
	switch {
	case failed == nil:
		return nil
	case errors.As(failed, &invalid):
		return &InvalidRequest{Message: invalid.Message, Field: invalid.Field, ErrorNumber: invalid.Number()}
	case errors.As(failed, &unauthenticated):
		return &Unauthenticated{Message: failed.Error(), ErrorNumber: unauthenticated.Number()}
	case errors.As(failed, &forbidden):
		return &Forbidden{Message: failed.Error(), ErrorNumber: forbidden.Number()}
	case errors.As(failed, &missing):
		return &NotFound{Message: failed.Error(), ErrorNumber: missing.Number()}
	case errors.As(failed, &conflict):
		return &Conflict{Message: failed.Error(), ErrorNumber: conflict.Number()}
	case errors.As(failed, &duplicate):
		return &Duplicate{Message: failed.Error(), ErrorNumber: duplicate.Number()}
	case errors.As(failed, &unavailable):
		return &Unavailable{Message: Unavailable{}.ChannelRouteFailed().Message, ErrorNumber: unavailable.Number()}
	}
	return Unavailable{}.ChannelRouteFailed()
}

func (service *ErrorMappingService) outboxError(failed error) error {
	missing := &channels.NotFound{}
	unavailable := &channels.Unavailable{}
	switch {
	case failed == nil:
		return nil
	case errors.As(failed, &missing):
		return &NotFound{Message: failed.Error(), ErrorNumber: missing.Number()}
	case errors.As(failed, &unavailable):
		return &Unavailable{Message: Unavailable{}.OutboxRouteFailed().Message, ErrorNumber: unavailable.Number()}
	}
	return Unavailable{}.OutboxRouteFailed()
}
