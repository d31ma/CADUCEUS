import (
	"os"

	"mesail.dev/packages/realtime"
)

// @Service
type PresenceService struct {
	// @Needs
	dataSource DataSource
	// @Needs
	failures *ErrorMappingService
}

func (service *PresenceService) Set(authorization string, request PresenceRequest) error {
	failed := service.desk().Set(authorization, realtime.PresenceRequest{State: request.State, DoNotDisturbUntil: request.DoNotDisturbUntil})
	return service.failures.PresenceError(failed)
}

func (service *PresenceService) Query(authorization string, memberIds string) ([]Presence, error) {
	found, failed := service.desk().Query(authorization, memberIds)
	return service.answers(found), service.failures.PresenceError(failed)
}

func (service *PresenceService) desk() realtime.PresenceDesk {
	return realtime.DeskFactory{Environment: os.Getenv}.Presence(StorageUtils{}.From(service.dataSource))
}

func (service *PresenceService) answers(found []realtime.Presence) []Presence {
	answers := []Presence{}
	for _, presence := range found {
		answers = append(answers, Presence{MemberId: presence.MemberId, State: presence.State, DoNotDisturbUntil: presence.DoNotDisturbUntil, LastSeenAt: presence.LastSeenAt})
	}
	return answers
}
