import "cmp"

// @Service
type JobRouteService struct {
	// @Needs
	callers *CallerService
	// @Needs
	removals *RemovalJobService
}

func (service *JobRouteService) Removals(authorization string) (RemovalReport, error) {
	caller, refused := service.callers.Caller(authorization)
	return service.removalsUnless(cmp.Or(refused, service.callers.InstallWide(caller)))
}

func (service *JobRouteService) removalsUnless(refused error) (RemovalReport, error) {
	if refused != nil {
		return RemovalReport{}, refused
	}
	return service.removals.RunOnce()
}
