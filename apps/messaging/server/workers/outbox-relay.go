import "mesail.dev/packages/adapters"

// @Schedule({"fixedDelaySeconds":60,"initialDelaySeconds":30})
type Handler struct{}

func (Handler) POST() {
	adapters.ScheduledSweep{}.Run()
}
