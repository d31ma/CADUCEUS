package channels

import "time"

type Ports struct {
	Documents Documents
	Relay     Relay
	Directory Directory
	Callers   Callers
	Settings  Settings
	Now       func() time.Time
}
