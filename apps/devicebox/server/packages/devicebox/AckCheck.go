package devicebox

import "slices"

type AckCheck struct{}

func (check AckCheck) Checked(envelopeIds []string) ([]string, error) {
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

func (AckCheck) count(envelopeIds []string) error {
	if len(envelopeIds) < 1 || len(envelopeIds) > (Limits{}).MaxAck() {
		return InvalidRequest{}.AckCount()
	}
	return nil
}

func (check AckCheck) formats(envelopeIds []string) error {
	if slices.ContainsFunc(envelopeIds, check.invalid) {
		return InvalidRequest{}.AckId()
	}
	return nil
}

func (AckCheck) invalid(envelopeId string) bool {
	_, valid := Formats{}.EnvelopeId(envelopeId)
	return !valid
}

func (AckCheck) normalized(envelopeIds []string) []string {
	normalized := []string{}
	for _, envelopeId := range envelopeIds {
		lowered, _ := Formats{}.EnvelopeId(envelopeId)
		normalized = append(normalized, lowered)
	}
	slices.Sort(normalized)
	return slices.Compact(normalized)
}
