// @RequestRoute("/api/v1/mail-in/health")
// @Public
type HealthController struct {
}

// @GetRoute
func (controller *HealthController) GetHealth() Health {
	return Health{Service: "mail-in", Status: "ok"}
}
