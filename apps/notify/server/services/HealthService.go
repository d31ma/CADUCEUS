// @Service
type HealthService struct {
}

func (service *HealthService) Health() Health {
	return Health{Service: "notify", Status: "ok"}
}
