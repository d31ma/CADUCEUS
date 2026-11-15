import (
	"errors"

	"mesail.dev/packages/backups"
	"mesail.dev/packages/events"
)

// @Service
type BackupFailureService struct {
	// @Needs
	failures *FailureService
}

func (service *BackupFailureService) Translated(failed error) error {
	return service.translated(failed)
}

func (service *BackupFailureService) TranslatedEvent(failed error) error {
	return service.translatedEvent(failed)
}

func (service *BackupFailureService) translated(failed error) error {
	invalid := backups.Invalid{}
	conflict := backups.Conflict{}
	switch {
	case errors.As(failed, &invalid):
		return &InvalidUpload{Field: invalid.Field, Reason: invalid.Reason, ErrorNumber: invalid.ErrorNumber}
	case errors.As(failed, &conflict):
		return &AttachmentConflict{Field: conflict.Field, Reason: conflict.Reason, ErrorNumber: conflict.ErrorNumber}
	case errors.Is(failed, backups.Missing{}):
		return &AttachmentNotFound{Message: "No backup was found.", ErrorNumber: 3514}
	}
	return service.failures.Translated(failed)
}

func (service *BackupFailureService) translatedEvent(failed error) error {
	switch {
	case errors.As(failed, &events.NotService{}):
		return &Forbidden{Message: "Only a service may send events.", ErrorNumber: 3521}
	case errors.Is(failed, backups.OtherDomain{}):
		return &Forbidden{Message: "A service may send events only for its own domain.", ErrorNumber: 3522}
	case errors.As(failed, &events.NotSubscribed{}), errors.As(failed, &events.InvalidEnvelope{}):
		return &InvalidUpload{Field: "envelope", Reason: "The event is not valid.", ErrorNumber: 3523}
	}
	return service.failures.Translated(failed)
}
