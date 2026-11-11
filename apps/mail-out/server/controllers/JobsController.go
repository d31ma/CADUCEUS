import (
	"os"

	"mesail.dev/packages/mailout"
)

// @RequestRoute("/api/v1/mail-out/jobs")
// @Allow(Service)
type JobsController struct {
	// @Needs
	dataSource DataSource
}

// @PostRoute("/deliver")
func (controller *JobsController) PostDeliver() (QueueReport, error) {
	report, failed := mailout.DeskFactory{Environment: os.Getenv}.Desk(controller.dataSource).Deliver()
	return QueueReport{Picked: report.Picked, Sent: report.Sent, Deferred: report.Deferred, Bounced: report.Bounced, Expired: report.Expired, Failed: report.Failed}, failed
}
