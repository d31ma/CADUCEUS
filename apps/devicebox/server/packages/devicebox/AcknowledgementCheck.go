package devicebox

import "slices"

type AcknowledgementCheck struct{}

func (check AcknowledgementCheck) Checked(envelopeIds []string) ([]string, error) {
	err := check.count(envelopeIds)
	if err != nil {
		return nil, err
	}
	err = check.formats(envelopeIds)
	if err != nil {
		return nil, err
	}
	return check.normalized(envelopeIds), nil
}

func (AcknowledgementCheck) count(envelopeIds []string) error {
	if len(envelopeIds) < 1 || len(envelopeIds) > (Limits{}).MaxAcknowledged() {
		return InvalidRequest{}.AcknowledgedCount()
	}
	return nil
}

func (check AcknowledgementCheck) formats(envelopeIds []string) error {
	if slices.ContainsFunc(envelopeIds, check.invalid) {
		return InvalidRequest{}.AcknowledgedId()
	}
	return nil
}

func (AcknowledgementCheck) invalid(envelopeId string) bool {
	_, valid := Formats{}.EnvelopeId(envelopeId)
	return !valid
}

func (AcknowledgementCheck) normalized(envelopeIds []string) []string {
	normalized := []string{}
	for _, envelopeId := range envelopeIds {
		lowered, _ := Formats{}.EnvelopeId(envelopeId)
		normalized = append(normalized, lowered)
	}
	slices.Sort(normalized)
	return slices.Compact(normalized)
}
