import "mesail.dev/packages/channels"

// @Schedule({"fixedDelaySeconds":60,"initialDelaySeconds":30})
type Handler struct{}

func (Handler) POST() {
	channels.NewScheduledSweep().Run()
}
