import (
	"errors"

	"mesail.dev/packages/mailin"
)

// @Service
type FailureService struct {
}

func (service *FailureService) Translate(failed error) error {
	return service.translated(failed)
}

func (service *FailureService) translated(failed error) error {
	invalid := mailin.Invalid{}
	missing := mailin.Missing{}
	switch {
	case failed == nil:
		return nil
	case errors.As(failed, &invalid):
		return &InvalidEnvelope{Field: invalid.Field, Reason: invalid.Reason, ErrorNumber: invalid.Number()}
	case errors.As(failed, &missing):
		return &NotFound{Message: missing.Message, ErrorNumber: missing.Number()}
	case errors.Is(failed, mailin.Unauthenticated{}):
		return &Unauthenticated{}
	}
	return &StorageUnavailable{Operation: failed.Error()}
}
