import (
	"errors"
	"os"

	"mesail.dev/packages/realtime"
)

// @RequestRoute("/api/v1/realtime/jobs")
// @Allow(Service)
type JobsController struct {
	// @Needs
	dataSource DataSource
}

// @PostRoute("/sweep")
func (controller *JobsController) PostSweep(
	// @FromHeader("authorization")
	authorization string,
) (SweepReport, error) {
	report, failed := controller.sweeper().Run(authorization)
	return SweepReport{ClosedConnections: report.ClosedConnections, OfflineMembers: report.OfflineMembers, EndedDoNotDisturb: report.EndedDoNotDisturb, DeletedTickets: report.DeletedTickets, Writes: report.Writes, More: report.More}, controller.failure(failed)
}

func (controller *JobsController) sweeper() realtime.Sweeper {
	return realtime.DeskFactory{Environment: os.Getenv}.Sweeper(StorageUtils{}.From(controller.dataSource))
}

func (controller *JobsController) failure(failed error) error {
	switch {
	case failed == nil:
		return nil
	case errors.As(failed, &realtime.Missing{}):
		return &NotFound{}
	case errors.Is(failed, realtime.Forbidden{}):
		return &Forbidden{}
	case errors.Is(failed, realtime.Unauthenticated{}):
		return &Unauthenticated{}
	}
	return &Unavailable{}
}
