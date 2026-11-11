import (
	"os"
	"time"

	"mesail.dev/packages/oidc"
	"mesail.dev/packages/transport"
	"mesail.dev/packages/trigger"
)

// @Schedule({"fixedDelaySeconds":60,"initialDelaySeconds":45})
type Handler struct{}

func (handler Handler) POST() {
	authorization, failure := oidc.ServiceTokensFactory{}.Configured().Authorization(handler.domain())
	route := trigger.LocalRoute{Client: transport.SingleUseTransport{}.Client(handler.timeout())}
	route.Post(os.Getenv("YON_APPLICATION_JSON"), handler.path(), authorization, failure)
}

func (Handler) domain() string {
	return "mesail-operators"
}

func (Handler) path() string {
	return "/api/v1/directory/jobs/removals"
}

func (Handler) timeout() time.Duration {
	return 60 * time.Second
}
