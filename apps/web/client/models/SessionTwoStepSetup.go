type SessionTwoStepSetup struct {
	Secret   string `json:"secret"`
	AppLink  string `json:"otpauthUri"`
	QrImage  string `json:"qrImage"`
	SetupKey string `json:"setupKey"`
}
