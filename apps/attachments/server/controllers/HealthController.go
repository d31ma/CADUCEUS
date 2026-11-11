// @RequestRoute("/api/v1/attachments/health")
// @Public
type HealthController struct {
}

// @GetRoute
func (controller *HealthController) GetHealth() Health {
	return Health{Service: "attachments", Status: "ok"}
}
