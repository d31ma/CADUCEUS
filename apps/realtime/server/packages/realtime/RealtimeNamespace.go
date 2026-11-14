package realtime

type RealtimeNamespace struct {
	Gateway      GatewayNamespace `json:"gateway"`
	DirectoryUrl string           `json:"directoryurl"`
	NotifyUrl    string           `json:"notifyurl"`
	Calls        CallKeySettings  `json:"calls"`
}
