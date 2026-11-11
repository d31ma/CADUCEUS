package dnscheck

import "slices"

type dohResponse struct {
	Status int         `json:"Status"`
	Answer []dohAnswer `json:"Answer"`
}

func (response dohResponse) check(name string) error {
	if slices.Contains(response.acceptedStatuses(), response.Status) {
		return nil
	}
	return LookupFailed{Name: name, Status: response.Status}
}

func (response dohResponse) values(kind recordType) []string {
	values := []string{}
	for _, answer := range response.Answer {
		values = append(values, answer.dataIfType(kind.number)...)
	}
	return values
}

func (response dohResponse) acceptedStatuses() []int {
	return []int{response.noError(), response.nameMissing()}
}

func (dohResponse) noError() int {
	return 0
}

func (dohResponse) nameMissing() int {
	return 3
}
