// @RequestRoute("/api/v1/messaging/jobs")
// @Allow(Service)
type JobsController struct {
	// @Needs
	changes *ChangeJobService
}

// @PostRoute("/changes-cleanup")
func (controller *JobsController) PostChangesCleanup(
	// @FromHeader("authorization")
	authorization string,
) (CleanupReport, error) {
	return controller.changes.Cleanup(authorization)
}
