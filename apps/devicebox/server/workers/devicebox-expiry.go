import (
	"os"

	"mesail.dev/packages/devicebox"
)

// @Schedule({"fixedDelaySeconds":60,"initialDelaySeconds":45})
type Handler struct{}

func (handler Handler) POST() {
	devicebox.ScheduledCall{Path: handler.path()}.Run(os.Getenv("YON_APPLICATION_JSON"))
}

func (Handler) path() string {
	return "/api/v1/devicebox/jobs/expire"
}
