// @RequestRoute("/api/v1/notify/jobs")
// @Allow(Service)
type JobsController struct {
	// @Needs
	jobs *JobsService
}

// @PostRoute("/run")
func (controller *JobsController) PostRun(
	// @FromHeader("authorization")
	authorization string,
) (JobReport, error) {
	return controller.jobs.Run(authorization)
}
