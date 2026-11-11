import "mesail.dev/packages/events"

// @Service
type MessageCreatedService struct{}

func (service *MessageCreatedService) Apply(domainId string, payload events.MessageCreated) error {
	return nil
}
