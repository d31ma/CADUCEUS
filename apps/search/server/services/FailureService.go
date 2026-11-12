import (
	"errors"

	"mesail.dev/packages/search"
)

// @Service
type FailureService struct {
}

func (service *FailureService) Translated(failed error) error {
	return service.translated(failed)
}

func (service *FailureService) translated(failed error) error {
	invalid := search.InvalidRequest{}
	forbidden := search.Forbidden{}
	switch {
	case failed == nil:
		return nil
	case errors.As(failed, &invalid):
		return &InvalidRequest{Message: invalid.Message, Field: invalid.Field, ErrorNumber: invalid.ErrorNumber}
	case errors.As(failed, &search.Unauthenticated{}):
		return &Unauthenticated{Message: failed.Error()}
	case errors.As(failed, &forbidden):
		return &Forbidden{Message: forbidden.Message, ErrorNumber: forbidden.ErrorNumber}
	}
	return &Unavailable{Message: failed.Error()}
}
