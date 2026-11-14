// @Service
type HealthService struct {
}

func (service *HealthService) Health() Health {
	return Health{Service: "devicebox", Status: "ok"}
}
