package oidc

type TotpSetup struct {
	Secret   string
	Uri      string
	QrImage  string
	SetupKey string
}
