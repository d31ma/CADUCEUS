// @RequestRoute("/api/v1/search/health")
// @Public
type HealthController struct {
}

// @GetRoute
func (controller *HealthController) GetHealth() Health {
	return Health{Service: "search", Status: "ok"}
}
