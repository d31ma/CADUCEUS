// @RequestRoute("/api/v1/channels/health")
// @Public
type HealthController struct {
}

// @GetRoute
func (controller *HealthController) GetHealth() Health {
	return Health{Service: "channels", Status: "ok"}
}
