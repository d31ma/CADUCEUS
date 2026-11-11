package dnscheck

type recordTypes struct{}

func (recordTypes) mx() recordType {
	return recordType{name: "MX", number: 15}
}

func (recordTypes) txt() recordType {
	return recordType{name: "TXT", number: 16}
}
