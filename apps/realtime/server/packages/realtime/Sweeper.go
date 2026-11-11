package realtime

import "mesail.dev/packages/oidc"

type Sweeper struct {
	Gate        oidc.Gate
	Domains     DomainBook
	Connections ConnectionBook
	Pushes      ConnectionGateway
	Presence    PresenceKeeper
}

func (sweeper Sweeper) Run(authorization string) (SweepReport, error) {
	return SweepReport{}, nil
}
