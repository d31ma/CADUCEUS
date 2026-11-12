// @Service
type HealthService struct {
}

func (service *HealthService) Health() Health {
	return Health{Service: "mail-out", Status: "ok"}
}
