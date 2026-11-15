import "mesail.dev/packages/channels"

// @Schedule({"fixedDelaySeconds":60,"initialDelaySeconds":82})
type Handler struct{}

func (Handler) POST() {
	channels.NewScheduledSweep().Run()
}
