package webpush

type RefusedEndpoint struct {
	Reason string
}

func (refused RefusedEndpoint) Error() string {
	return "the push endpoint is refused: " + refused.Reason
}
