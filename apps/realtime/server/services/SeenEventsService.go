// @Service
type SeenEventsService struct {
	// @Needs
	dataSource DataSource
}

func (service *SeenEventsService) Seen(domainId string, eventId string) (bool, error) {
	return false, nil
}

func (service *SeenEventsService) Remember(domainId string, eventId string) error {
	return nil
}
