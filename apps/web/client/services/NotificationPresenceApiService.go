import "strings"

// @Service
type NotificationPresenceApiService struct {
	// @Needs
	presence *PresenceClient
	// @Needs
	access *NotificationTokenService
	// @Needs
	problems *NotificationProblemService
}

func (service *NotificationPresenceApiService) Announce(request PresenceRequest) string {
	_, failure := service.access.Send(func(authorization string) (BehaviorValue, error) {
		return service.presence.Announce(authorization, request)
	})
	return service.problems.OfFailure(failure)
}

func (service *NotificationPresenceApiService) Find(memberIds []string) ([]Presence, string) {
	presences, failure := service.findWhenAsked(memberIds)
	return presences, service.problems.OfFailure(failure)
}

func (service *NotificationPresenceApiService) findWhenAsked(memberIds []string) ([]Presence, error) {
	if len(memberIds) == 0 {
		return []Presence{}, nil
	}
	return BehaviorsAnswer[[]Presence](service.access.Send(func(authorization string) (BehaviorValue, error) {
		return service.presence.Find(authorization, strings.Join(memberIds, ","))
	}))
}
