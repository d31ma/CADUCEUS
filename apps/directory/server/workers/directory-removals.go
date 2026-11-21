import (
	"os"

	"mesail.dev/packages/scheduling"
	"mesail.dev/packages/trigger"
)

// @Schedule({"fixedDelaySeconds":60,"initialDelaySeconds":88})
type Handler struct{}

func (handler Handler) POST() {
	settings := os.Getenv("YON_APPLICATION_JSON")
	scheduling.WorkerSwitch{ApplicationJson: settings}.Run(func() { trigger.InstallWideJob{Path: handler.path()}.Run(settings) })
}

func (Handler) path() string {
	return "/api/v1/directory/jobs/removals"
}
