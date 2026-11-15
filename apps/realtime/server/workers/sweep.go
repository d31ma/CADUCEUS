import (
	"os"

	"mesail.dev/packages/realtime"
)

// @Schedule({"fixedDelaySeconds":15,"initialDelaySeconds":17})
type Handler struct{}

func (Handler) POST() error {
	return realtime.SweepTrigger{}.Configured(os.Getenv).Fire()
}
