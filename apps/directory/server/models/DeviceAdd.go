type DeviceAdd struct {
	DeviceId       string `json:"deviceId"`
	Name           string `json:"name"`
	IdentityKey    string `json:"identityKey"`
	AgreementKey   string `json:"agreementKey"`
	BackupKey      string `json:"backupKey,omitempty"`
	SignerDeviceId string `json:"signerDeviceId,omitempty"`
	Signature      string `json:"signature,omitempty"`
}
