package mailin

import "mesail.dev/packages/outbox"

type EventPatch struct {
	outbox.Fields
	EventQueued bool `json:"eventQueued"`
}
