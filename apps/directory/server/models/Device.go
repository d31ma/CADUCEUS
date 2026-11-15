type Device struct {
	DeviceId     string `json:"deviceId"`
	PersonId     string `json:"personId"`
	Name         string `json:"name,omitempty"`
	IdentityKey  string `json:"identityKey"`
	AgreementKey string `json:"agreementKey"`
	AddedAt      string `json:"addedAt"`
}
