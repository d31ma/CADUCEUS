// @RequestRoute("/api/v1/realtime/health")
// @Public
type HealthController struct {
}

// @GetRoute
func (controller *HealthController) GetHealth() Health {
	return Health{Service: "realtime", Status: "ok"}
}
