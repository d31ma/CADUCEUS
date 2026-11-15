// @RequestRoute("/api/v1/directory/jobs")
// @Allow(Service)
type JobsController struct {
	// @Needs
	jobs *JobRouteService
}

// @PostRoute("/removals")
func (controller *JobsController) PostRemovals(
	// @FromHeader("authorization")
	authorization string,
) (RemovalReport, error) {
	return controller.jobs.Removals(authorization)
}
