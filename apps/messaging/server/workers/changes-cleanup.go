import "mesail.dev/packages/adapters"

// @Schedule({"fixedDelaySeconds":86400,"initialDelaySeconds":600})
type Handler struct{}

func (Handler) POST() {
	adapters.ScheduledCleanup{}.Run()
}
