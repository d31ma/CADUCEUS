package trigger

import (
	"time"

	"mesail.dev/packages/oidc"
	"mesail.dev/packages/transport"
)

type InstallWideJob struct {
	Path string
}

func (job InstallWideJob) Run(applicationJson string) error {
	authorization, failure := oidc.ServiceTokensFactory{}.FromJson(applicationJson).Authorization(oidc.InstallWide{}.DomainId())
	route := LocalRoute{Client: transport.SingleUseTransport{}.Client(job.timeout())}
	return route.Post(applicationJson, job.Path, authorization, failure)
}

func (InstallWideJob) timeout() time.Duration {
	return 60 * time.Second
}
