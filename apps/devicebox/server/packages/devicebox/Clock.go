package devicebox

import "time"

type Clock interface {
	Now() time.Time
}
