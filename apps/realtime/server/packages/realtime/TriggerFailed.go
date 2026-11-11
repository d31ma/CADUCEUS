package realtime

type TriggerFailed struct {
	Reason string
}

func (failure TriggerFailed) Error() string {
	return "the sweep trigger failed: " + failure.Reason
}
