import (
	"os"

	"mesail.dev/packages/channels"
	"mesail.dev/packages/scheduling"
)

// @Schedule({"fixedDelaySeconds":60,"initialDelaySeconds":82})
type Handler struct{}

func (Handler) POST() {
	scheduling.WorkerSwitch{ApplicationJson: os.Getenv("YON_APPLICATION_JSON")}.Run(func() { channels.NewScheduledSweep().Run() })
}
