import (
	"os"

	"mesail.dev/packages/adapters"
	"mesail.dev/packages/scheduling"
)

// @Schedule({"fixedDelaySeconds":60,"initialDelaySeconds":37})
type Handler struct{}

func (Handler) POST() {
	scheduling.WorkerSwitch{ApplicationJson: os.Getenv("YON_APPLICATION_JSON")}.Run(func() { adapters.ScheduledSweep{}.Run() })
}
