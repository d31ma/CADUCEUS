// @RequestRoute("/api/v1/mail-out/health")
// @Public
type HealthController struct {
}

// @GetRoute
func (controller *HealthController) GetHealth() Health {
	return Health{Service: "mail-out", Status: "ok"}
}
