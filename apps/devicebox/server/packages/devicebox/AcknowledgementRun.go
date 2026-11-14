package devicebox

type AcknowledgementRun struct {
	store Store
}

func NewAcknowledgementRun(store Store) AcknowledgementRun {
	return AcknowledgementRun{store: store}
}

func (run AcknowledgementRun) Remove(domainId string, deviceKey string, envelopeIds []string) (int, error) {
	exists, err := run.store.HasDomain(domainId)
	if err != nil || !exists {
		return 0, err
	}
	removed := 0
	for _, envelopeId := range envelopeIds {
		count, err := run.removeOne(domainId, deviceKey, envelopeId)
		removed += count
		if err != nil {
			return removed, err
		}
	}
	return removed, nil
}

func (run AcknowledgementRun) removeOne(domainId string, deviceKey string, envelopeId string) (int, error) {
	slip, found, err := run.store.SlipByEnvelope(domainId, envelopeId)
	if err != nil || !found || !slip.Waiting[deviceKey] {
		return 0, err
	}
	return 1, run.without(domainId, deviceKey, slip)
}

func (run AcknowledgementRun) without(domainId string, deviceKey string, slip Slip) error {
	delete(slip.Waiting, deviceKey)
	if len(slip.Waiting) == 0 {
		return run.store.DeleteEnvelope(domainId, slip)
	}
	return run.store.SaveWaiting(domainId, slip)
}
