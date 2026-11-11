// @RequestRoute("/api/v1/messaging/health")
// @Public
type HealthController struct {
}

// @GetRoute
func (controller *HealthController) GetHealth() Health {
	return Health{Service: "messaging", Status: "ok"}
}
