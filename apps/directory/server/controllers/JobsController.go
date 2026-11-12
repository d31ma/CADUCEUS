import "cmp"

// @RequestRoute("/api/v1/directory/jobs")
// @Allow(Service)
type JobsController struct {
	// @Needs
	callers *CallerService
	// @Needs
	removals *RemovalJobService
}

// @PostRoute("/removals")
func (controller *JobsController) PostRemovals(
	// @FromHeader("authorization")
	authorization string,
) (RemovalReport, error) {
	caller, refused := controller.callers.Caller(authorization)
	return controller.removalsUnless(cmp.Or(refused, controller.callers.InstallWide(caller)))
}

func (controller *JobsController) removalsUnless(refused error) (RemovalReport, error) {
	if refused != nil {
		return RemovalReport{}, refused
	}
	return controller.removals.RunOnce()
}
