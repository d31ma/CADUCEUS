// @Service
type HealthService struct {
}

func (service *HealthService) Health() Health {
	return Health{Service: "realtime", Status: "ok"}
}
