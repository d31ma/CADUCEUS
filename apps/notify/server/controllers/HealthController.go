// @RequestRoute("/api/v1/notify/health")
// @Public
type HealthController struct {
}

// @GetRoute
func (controller *HealthController) GetHealth() Health {
	return Health{Service: "notify", Status: "ok"}
}
