package dnscheck

type dohAnswer struct {
	Type int    `json:"type"`
	Data string `json:"data"`
}

func (answer dohAnswer) dataIfType(number int) []string {
	if answer.Type == number {
		return []string{answer.Data}
	}
	return nil
}
