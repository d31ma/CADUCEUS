// @Service
type DeliveryService struct {
	// @Needs
	desks *DeskService
}

func (service *DeliveryService) Deliver() (QueueReport, error) {
	report, failed := service.desks.Desk().Deliver()
	return QueueReport{Picked: report.Picked, Sent: report.Sent, Deferred: report.Deferred, Bounced: report.Bounced, Expired: report.Expired, Failed: report.Failed}, failed
}
