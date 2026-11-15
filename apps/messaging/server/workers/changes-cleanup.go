import "mesail.dev/packages/adapters"

// @Schedule({"fixedDelaySeconds":86400,"initialDelaySeconds":613})
type Handler struct{}

func (Handler) POST() {
	adapters.ScheduledCleanup{}.Run()
}
