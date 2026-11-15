import (
	"time"

	"mesail.dev/packages/oidc"
	"mesail.dev/packages/storage"
)

// @Service
type ChangeJobService struct {
	// @Needs
	storage *StorageService
	// @Needs
	failures *FailureService
	// @Needs
	wire *WireService
}

func (service *ChangeJobService) Cleanup(authorization string) (CleanupReport, error) {
	report, err := service.cleanupUnless(service.installWideOnly(authorization))
	return service.wire.CleanupReport(report), service.failures.Known(err)
}

func (service *ChangeJobService) installWideOnly(authorization string) error {
	caller, failure := oidc.Configured().Identify(authorization)
	if failure == nil && (oidc.InstallWide{}).Matches(caller.DomainId) {
		return nil
	}
	return Forbidden{}.JobsNotInstallWide()
}

func (service *ChangeJobService) cleanupUnless(refused error) (storage.CleanupReport, error) {
	if refused != nil {
		return storage.CleanupReport{}, refused
	}
	return service.storage.Storage().ChangeLogs().Cleanup(time.Now())
}
