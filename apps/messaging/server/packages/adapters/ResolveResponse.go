package adapters

import "mesail.dev/packages/core"

type ResolveResponse struct {
	Recipients []core.Resolved `json:"recipients"`
}
