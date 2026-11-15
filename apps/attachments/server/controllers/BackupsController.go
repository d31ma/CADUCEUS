// @RequestRoute("/api/v1/attachments/backups")
// @SignedIn
type BackupsController struct {
	// @Needs
	backups *BackupService
}

// @GetRoute("/mine")
func (controller *BackupsController) GetMine(
	// @FromHeader("authorization")
	authorization string,
) (Backup, error) {
	return controller.backups.Mine(authorization)
}

// @PutRoute("/mine")
// @ResponseStatus(201)
func (controller *BackupsController) PutMine(
	// @FromHeader("authorization")
	authorization string,
	// @FromBody
	request BackupStart,
) (BackupUpload, error) {
	return controller.backups.Start(authorization, request)
}

// @PostRoute("/mine/complete")
func (controller *BackupsController) PostComplete(
	// @FromHeader("authorization")
	authorization string,
	// @FromBody
	request BackupComplete,
) (Backup, error) {
	return controller.backups.Complete(authorization, request)
}

// @GetRoute("/mine/download")
func (controller *BackupsController) GetDownload(
	// @FromHeader("authorization")
	authorization string,
) (Download, error) {
	return controller.backups.Download(authorization)
}

// @DeleteRoute("/mine")
// @ResponseStatus(204)
func (controller *BackupsController) DeleteMine(
	// @FromHeader("authorization")
	authorization string,
) error {
	return controller.backups.Delete(authorization)
}
