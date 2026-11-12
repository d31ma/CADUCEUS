// @RequestRoute("/api/v1/mail-out/health")
// @Public
type HealthController struct {
	// @Needs
	health *HealthService
}

// @GetRoute
func (controller *HealthController) GetHealth() Health {
	return controller.health.Health()
}
