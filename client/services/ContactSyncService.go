import "slices"

// ContactSyncService follows a queued CardDAV change until the server settles
// it: it asks for the mutation's state now and then every half second, up to
// sixteen times, and announces the outcome in ContactMutationSettled. A change
// that does not settle in time stays queued for synchronization.
// @Service
type ContactSyncService struct {
	// @Needs
	contactsClient            *ContactsClient
	applicationEventPublisher ApplicationEventPublisher
	Pending                   bool
	token                     string
	mutation                  ContactMutationStarted
	attempts                  int
}

// HearSession keeps the bearer token.
// @OnEvent({"replay":true})
func (sync *ContactSyncService) HearSession(changed SessionChanged) {
	sync.token = changed.Token
}

// HearStarted follows a new CardDAV change.
// @OnEvent
func (sync *ContactSyncService) HearStarted(started ContactMutationStarted) {
	sync.mutation = started
	sync.attempts = 0
	sync.Pending = true
	sync.poll()
}

// HearTimer asks again after the pause.
// @OnEvent
func (sync *ContactSyncService) HearTimer(elapsed TimerElapsed) {
	FlowUtils{}.When(!slices.Contains([]bool{elapsed.Name == sync.timer(), sync.Pending}, false), sync.poll)()
}

func (sync *ContactSyncService) poll() {
	sync.attempts++
	answer, failure := sync.contactsClient.Mutation("Bearer "+sync.token, sync.mutation.ContactID, sync.mutation.MutationID, sync.mutation.AccountID)
	sync.next(sync.settledState(answer, failure))()
}

// settledState is the mutation's final state, or "" while it is still open
// or could not be read.
func (ContactSyncService) settledState(answer ContactMutation, failure error) string {
	if failure != nil {
		return ""
	}
	return map[string]string{"applied": answer.State, "conflict": answer.State, "failed": answer.State}[answer.State]
}

func (sync *ContactSyncService) next(state string) func() {
	switch {
	case state != "":
		return func() { sync.settle(state) }
	case sync.attempts >= sync.limit():
		return sync.giveUp
	default:
		return sync.wait
	}
}

func (sync *ContactSyncService) wait() {
	sync.applicationEventPublisher.PublishEvent(TimerRequested{Name: sync.timer(), DelayMs: 500}, false)
}

func (sync *ContactSyncService) giveUp() {
	sync.applicationEventPublisher.PublishEvent(ToastRequested{Message: "Contact change is queued for synchronization."}, false)
	sync.settle("queued")
}

func (sync *ContactSyncService) settle(state string) {
	sync.Pending = false
	sync.applicationEventPublisher.PublishEvent(ContactMutationSettled{
		ContactID: sync.mutation.ContactID,
		Kind:      sync.mutation.Kind,
		State:     state,
		Conflict:  sync.mutation.Conflict,
	}, false)
}

func (ContactSyncService) timer() string { return "contact-mutation" }

func (ContactSyncService) limit() int { return 16 }
