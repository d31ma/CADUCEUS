type MailOutSettings struct {
	MailInUrl         string `json:"mailinurl"`
	MessagingUrl      string `json:"messagingurl"`
	NotifyUrl         string `json:"notifyurl"`
	DirectoryUrl      string `json:"directoryurl"`
	RelayAddress      string `json:"relayaddress"`
	RelayUsername     string `json:"relayusername"`
	RelayPassword     string `json:"relaypassword"`
	RelayRequireTls   bool   `json:"relayrequiretls"`
	RelayImplicitTls  bool   `json:"relayimplicittls"`
	HeloName          string `json:"heloname"`
	DirectEnabled     bool   `json:"directenabled"`
	DirectMxHost      string `json:"directmxhost"`
	DirectPort        int    `json:"directport"`
	SecretFolder      string `json:"secretfolder"`
	SecretPrefix      string `json:"secretprefix"`
	BlobFolder        string `json:"blobfolder"`
	MaxAgeMinutes     int    `json:"maxageminutes"`
	FirstDelaySeconds int    `json:"firstdelayseconds"`
	MaxDelaySeconds   int    `json:"maxdelayseconds"`
	LeaseSeconds      int    `json:"leaseseconds"`
	TimeoutSeconds    int    `json:"timeoutseconds"`
}
