package admission

import "time"

type Clock interface {
	Now() time.Time
}
