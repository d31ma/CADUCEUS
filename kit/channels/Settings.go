package channels

import "mesail.dev/packages/outbox"

type Settings struct {
	DirectoryUrl string
	RelayUrl     string
	Outbox       outbox.Settings
	OutboxError  error
}
