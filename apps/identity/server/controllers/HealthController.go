import "mesail.dev/packages/login"

// @RequestRoute("/api/v1/identity/health")
// @Public
type HealthController struct {
}

// @GetRoute
func (controller *HealthController) GetHealth() Health {
	problem := login.SessionsFactory{}.PasskeyProblem()
	return Health{Service: "identity", Status: controller.status(problem)}
}

func (controller *HealthController) status(problem error) string {
	if problem != nil {
		return "degraded"
	}
	return "ok"
}
