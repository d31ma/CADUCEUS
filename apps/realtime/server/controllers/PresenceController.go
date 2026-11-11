import (
	"errors"
	"os"

	"mesail.dev/packages/realtime"
)

// @RequestRoute("/api/v1/realtime/presence")
// @SignedIn
type PresenceController struct {
	// @Needs
	dataSource DataSource
}

// @PutRoute
// @ResponseStatus(204)
func (controller *PresenceController) PutPresence(
	// @FromHeader("authorization")
	authorization string,
	// @FromBody
	request PresenceRequest,
) error {
	failed := controller.desk().Set(authorization, realtime.PresenceRequest{State: request.State, DoNotDisturbUntil: request.DoNotDisturbUntil})
	return controller.failure(failed)
}

// @GetRoute
func (controller *PresenceController) GetPresence(
	// @FromHeader("authorization")
	authorization string,
	// @FromQuery("memberIds")
	memberIds string,
) ([]Presence, error) {
	found, failed := controller.desk().Query(authorization, memberIds)
	return controller.answers(found), controller.failure(failed)
}

func (controller *PresenceController) desk() realtime.PresenceDesk {
	return realtime.DeskFactory{Environment: os.Getenv}.Presence(StorageUtils{}.From(controller.dataSource))
}

func (controller *PresenceController) answers(found []realtime.Presence) []Presence {
	answers := []Presence{}
	for _, presence := range found {
		answers = append(answers, Presence{MemberId: presence.MemberId, State: presence.State, DoNotDisturbUntil: presence.DoNotDisturbUntil, LastSeenAt: presence.LastSeenAt})
	}
	return answers
}

func (controller *PresenceController) failure(failed error) error {
	invalid := realtime.Invalid{}
	switch {
	case failed == nil:
		return nil
	case errors.As(failed, &invalid):
		return &InvalidRequest{Field: invalid.Field, Reason: invalid.Reason}
	case errors.Is(failed, realtime.Unauthenticated{}):
		return &Unauthenticated{}
	case errors.As(failed, &realtime.Missing{}):
		return &NotFound{}
	}
	return &Unavailable{}
}
