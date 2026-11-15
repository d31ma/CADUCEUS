// @Service
type BackupService struct {
	// @Needs
	desks *BackupDeskService
	// @Needs
	failures *BackupFailureService
	// @Needs
	views *BackupViewService
}

func (service *BackupService) Mine(authorization string) (Backup, error) {
	backup, failed := service.desks.Desk().Mine(authorization)
	return service.views.Backup(backup), service.failures.Translated(failed)
}

func (service *BackupService) Start(authorization string, request BackupStart) (BackupUpload, error) {
	upload, failed := service.desks.Desk().Start(authorization, service.views.Start(request))
	return BackupUpload{Version: upload.Version, UploadUrl: upload.UploadUrl, ExpiresAt: upload.ExpiresAt}, service.failures.Translated(failed)
}

func (service *BackupService) Complete(authorization string, request BackupComplete) (Backup, error) {
	backup, failed := service.desks.Desk().Complete(authorization, request.Version)
	return service.views.Backup(backup), service.failures.Translated(failed)
}

func (service *BackupService) Download(authorization string) (Download, error) {
	download, failed := service.desks.Desk().Download(authorization)
	return Download{Url: download.Url, ExpiresAt: download.ExpiresAt}, service.failures.Translated(failed)
}

func (service *BackupService) Delete(authorization string) error {
	return service.failures.Translated(service.desks.Desk().Delete(authorization))
}
