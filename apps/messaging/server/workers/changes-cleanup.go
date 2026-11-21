import (
	"os"

	"mesail.dev/packages/adapters"
	"mesail.dev/packages/scheduling"
)

// @Schedule({"fixedDelaySeconds":86400,"initialDelaySeconds":613})
type Handler struct{}

func (Handler) POST() {
	scheduling.WorkerSwitch{ApplicationJson: os.Getenv("YON_APPLICATION_JSON")}.Run(func() { adapters.ScheduledCleanup{}.Run() })
}
