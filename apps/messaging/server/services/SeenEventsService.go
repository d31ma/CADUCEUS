// @Service
type SeenEventsService struct {
	// @Needs
	tenant *TenantCollectionsService
	// @Needs
	names *CollectionNamesService
}

func (service *SeenEventsService) Seen(domainId string, eventId string) (bool, error) {
	return service.tenant.First(service.names.Seen(domainId), domainId, map[string]any{service.eventField(): eventId}, &SeenEvent{})
}

func (service *SeenEventsService) Remember(domainId string, eventId string) error {
	_, failure := service.tenant.Insert(service.names.Seen(domainId), SeenEvent{DomainId: domainId, EventId: eventId})
	return failure
}

func (service *SeenEventsService) eventField() string {
	return "eventId"
}
