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
	authorization, failure := service.access.Authorization()
	return service.problems.OfFailure(service.announce(authorization, request, failure))
}

func (service *NotificationPresenceApiService) Find(memberIds []string) ([]Presence, string) {
	authorization, failure := service.access.Authorization()
	presences, failure := service.find(authorization, memberIds, failure)
	return presences, service.problems.OfFailure(failure)
}

func (service *NotificationPresenceApiService) announce(authorization string, request PresenceRequest, failure error) error {
	if failure != nil {
		return failure
	}
	_, failure = service.presence.Announce(authorization, request)
	return failure
}

func (service *NotificationPresenceApiService) find(authorization string, memberIds []string, failure error) ([]Presence, error) {
	if failure != nil || len(memberIds) == 0 {
		return []Presence{}, failure
	}
	return BehaviorsAnswer[[]Presence](service.presence.Find(authorization, strings.Join(memberIds, ",")))
}
