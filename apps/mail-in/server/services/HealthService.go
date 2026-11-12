// @Service
type HealthService struct {
}

func (service *HealthService) Health() Health {
	return Health{Service: "mail-in", Status: "ok"}
}
