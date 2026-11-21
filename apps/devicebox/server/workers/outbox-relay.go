import (
	"os"

	"mesail.dev/packages/devicebox"
	"mesail.dev/packages/scheduling"
)

// @Schedule({"fixedDelaySeconds":60,"initialDelaySeconds":67})
type Handler struct{}

func (handler Handler) POST() {
	settings := os.Getenv("YON_APPLICATION_JSON")
	scheduling.WorkerSwitch{ApplicationJson: settings}.Run(func() { devicebox.ScheduledCall{Path: handler.path()}.Run(settings) })
}

func (Handler) path() string {
	return "/api/v1/devicebox/outbox/relay"
}
