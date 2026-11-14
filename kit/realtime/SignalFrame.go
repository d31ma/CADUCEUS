package realtime

type SignalFrame struct {
	Type         string `json:"type"`
	FromDeviceId string `json:"fromDeviceId"`
	FromPersonId string `json:"fromPersonId"`
	Kind         string `json:"kind"`
	Ciphertext   string `json:"ciphertext"`
}
