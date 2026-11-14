package realtime

type Settings struct {
	GatewayUrl    string
	GatewaySecret string
	DirectoryUrl  string
	NotifyUrl     string
	CallKeys      CallKeySettings
}
