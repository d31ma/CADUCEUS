type TotpSetup struct {
	Secret     string `json:"secret"`
	OtpauthUri string `json:"otpauthUri"`
	QrImage    string `json:"qrImage"`
	SetupKey   string `json:"setupKey"`
}
