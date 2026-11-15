import (
	"mesail.dev/packages/attachments"
	"mesail.dev/packages/backups"
	"mesail.dev/packages/events"
)

// @Test(HealthTestSetup, {"partition":"tests"})
type BackupFailureServiceTest struct {
	behaviors *BehaviorContext
}

func (test *BackupFailureServiceTest) TestNoFailureStaysNil() {
	failures := (&BackupFailureService{failures: &FailureService{}})
	Assert.Equals(nil, failures.Translated(nil))
	Assert.Equals(nil, failures.TranslatedEvent(nil))
}

func (test *BackupFailureServiceTest) TestBackupFailuresKeepTheirNumbers() {
	failures := (&BackupFailureService{failures: &FailureService{}})
	Assert.Equals(&InvalidUpload{Field: "deviceId", Reason: backups.Invalid{}.DeviceId().Reason, ErrorNumber: 3518}, failures.Translated(backups.Invalid{}.DeviceId()))
	Assert.Equals(&AttachmentConflict{Field: "file", Reason: backups.Conflict{}.NotStarted().Reason, ErrorNumber: 3520}, failures.Translated(backups.Conflict{}.NotStarted()))
	Assert.Equals(&AttachmentNotFound{Message: "No backup was found.", ErrorNumber: 3514}, failures.Translated(backups.Missing{}))
	Assert.Equals(&Unauthenticated{}, failures.Translated(attachments.Unauthenticated{}))
}

func (test *BackupFailureServiceTest) TestEventFailuresKeepTheirNumbers() {
	failures := (&BackupFailureService{failures: &FailureService{}})
	Assert.Equals(&Forbidden{Message: "Only a service may send events.", ErrorNumber: 3521}, failures.TranslatedEvent(events.NotService{}))
	Assert.Equals(&Forbidden{Message: "A service may send events only for its own domain.", ErrorNumber: 3522}, failures.TranslatedEvent(backups.OtherDomain{}))
	Assert.Equals(&InvalidUpload{Field: "envelope", Reason: "The event is not valid.", ErrorNumber: 3523}, failures.TranslatedEvent(events.NotSubscribed{Service: "attachments", Type: "x"}))
	Assert.Equals(&StorageUnavailable{Operation: attachments.Unavailable{Operation: "backup find"}.Error()}, failures.TranslatedEvent(attachments.Unavailable{Operation: "backup find"}))
}
