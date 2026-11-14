import (
	"os"

	"mesail.dev/packages/realtime"
)

// @Service
type SweepService struct {
	// @Needs
	dataSource DataSource
	// @Needs
	failures *ErrorMappingService
}

func (service *SweepService) Sweep(authorization string) (SweepReport, error) {
	report, failed := service.sweeper().Run(authorization)
	return service.report(report), service.failures.SweepError(failed)
}

func (service *SweepService) sweeper() realtime.Sweeper {
	return realtime.DeskFactory{Environment: os.Getenv}.Sweeper(StorageUtils{}.From(service.dataSource))
}

func (service *SweepService) report(report realtime.SweepReport) SweepReport {
	return SweepReport{ClosedConnections: report.ClosedConnections, OfflineMembers: report.OfflineMembers, EndedDoNotDisturb: report.EndedDoNotDisturb, DeletedTickets: report.DeletedTickets, Writes: report.Writes, More: report.More}
}
