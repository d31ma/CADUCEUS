import (
	"time"

	"mesail.dev/packages/oidc"
)

// @RequestRoute("/api/v1/mail-out/jobs")
// @Allow(Service)
type JobsController struct {
	// @Needs
	queue *QueueService
}

// @PostRoute("/deliver")
func (controller *JobsController) PostDeliver() (QueueReport, error) {
	return controller.queue.Run(time.Now(), oidc.ServiceTokensFactory{}.Configured().Authorization)
}
