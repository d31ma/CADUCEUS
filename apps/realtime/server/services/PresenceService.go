// @Service
type PresenceService struct {
	// @Needs
	presenceStoreService *PresenceStoreService
	// @Needs
	connectionStoreService *ConnectionStoreService
	// @Needs
	fanOutService *FanOutService
}

func (service *PresenceService) Set(domainId string, memberId string, request PresenceRequest) error {
	return nil
}

func (service *PresenceService) Connected(domainId string, memberId string) error {
	return nil
}

func (service *PresenceService) Disconnected(domainId string, memberId string) error {
	return nil
}

func (service *PresenceService) Query(domainId string, memberIds []string) ([]Presence, error) {
	return []Presence{}, nil
}

func (service *PresenceService) Settle(domainId string, now string, budget int) (SweepReport, error) {
	return SweepReport{}, nil
}
