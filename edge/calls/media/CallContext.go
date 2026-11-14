package media

import "mesail.dev/edge/calls/admission"

type CallContext struct {
	Options CallOptions
	Clock   admission.Clock
	Log     EventLog
}
