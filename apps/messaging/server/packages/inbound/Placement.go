package inbound

import "mesail.dev/packages/core"

type Placement struct {
	Conversation core.Conversation
	Root         core.Message
	HasRoot      bool
}
