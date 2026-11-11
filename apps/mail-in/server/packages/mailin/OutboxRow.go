package mailin

import "mesail.dev/packages/outbox"

type OutboxRow struct {
	Id string `json:"id"`
	outbox.Fields
}
