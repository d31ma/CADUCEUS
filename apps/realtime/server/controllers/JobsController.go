// @RequestRoute("/api/v1/realtime/jobs")
// @Allow(Service)
type JobsController struct {
	// @Needs
	sweeps *SweepService
}

// @PostRoute("/sweep")
func (controller *JobsController) PostSweep(
	// @FromHeader("authorization")
	authorization string,
) (SweepReport, error) {
	return controller.sweeps.Sweep(authorization)
}
