type DeviceDocument struct {
	Id           string `json:"id,omitempty"`
	DeviceId     string `json:"deviceId"`
	PersonId     string `json:"personId"`
	Name         string `json:"name"`
	IdentityKey  string `json:"identityKey"`
	AgreementKey string `json:"agreementKey"`
	AddedAt      string `json:"addedAt"`
}
