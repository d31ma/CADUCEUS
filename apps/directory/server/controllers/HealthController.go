// @RequestRoute("/api/v1/directory/health")
// @Public
type HealthController struct {
}

// @GetRoute
func (controller *HealthController) GetHealth() Health {
	return Health{Service: "directory", Status: "ok"}
}
