package core

import (
	"time"
)

type Ports struct {
	Directory   Directory
	Renderer    Renderer
	Inbound     InboundMail
	Attachments AttachmentCatalog
	Stager      Stager
	Now         func() time.Time
}
