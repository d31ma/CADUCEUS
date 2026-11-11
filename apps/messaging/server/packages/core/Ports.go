package core

import (
	"time"
)

type Ports struct {
	Directory Directory
	Renderer  Renderer
	Inbound   InboundMail
	Stager    Stager
	Now       func() time.Time
}
