package reading

import "mesail.dev/packages/core"

type messagePage struct {
	messages   []core.Message
	nextCursor string
}
