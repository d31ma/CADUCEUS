import (
	"os"

	"mesail.dev/packages/trigger"
)

// @Schedule({"fixedDelaySeconds":60,"initialDelaySeconds":45})
type Handler struct{}

func (handler Handler) POST() {
	trigger.InstallWideJob{Path: handler.path()}.Run(os.Getenv("YON_APPLICATION_JSON"))
}

func (Handler) path() string {
	return "/api/v1/directory/jobs/removals"
}
