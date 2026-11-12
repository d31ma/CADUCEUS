// @RequestRoute("/api/v1/mail-out/jobs")
// @Allow(Service)
type JobsController struct {
	// @Needs
	deliveries *DeliveryService
}

// @PostRoute("/deliver")
func (controller *JobsController) PostDeliver() (QueueReport, error) {
	return controller.deliveries.Deliver()
}
