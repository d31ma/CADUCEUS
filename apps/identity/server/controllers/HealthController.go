// @RequestRoute("/api/v1/identity/health")
// @Public
type HealthController struct {
}

// @GetRoute
func (controller *HealthController) GetHealth() Health {
	return Health{Service: "identity", Status: "ok"}
}
