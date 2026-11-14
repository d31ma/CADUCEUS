package events

type EnvelopeStored struct {
	ToPersonId  string   `json:"toPersonId"`
	ToDeviceIds []string `json:"toDeviceIds"`
	Kind        string   `json:"kind"`
	EnvelopeId  string   `json:"envelopeId"`
}
