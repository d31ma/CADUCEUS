import "mesail.dev/packages/login"

// @Service
type HealthService struct {
}

func (service *HealthService) Health() Health {
	problem := login.SessionsFactory{}.PasskeyProblem()
	return Health{Service: "identity", Status: service.status(problem)}
}

func (service *HealthService) status(problem error) string {
	if problem != nil {
		return "degraded"
	}
	return "ok"
}
