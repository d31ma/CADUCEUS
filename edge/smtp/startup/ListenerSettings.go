package startup

import "mesail.dev/edge/smtp/adapters"

type ListenerSettings struct {
	Address         string
	Hostname        string
	MaxSize         int64
	MaxConnections  int
	CertificateFile string
	KeyFile         string
	VerifyAuth      bool
	DirectoryUrl    string
	MailInUrl       string
	RawStoreFolder  string
	RawStoreS3      S3RawStoreSettings
	ServiceToken    string
	TokenSettings   adapters.TokenSettings
}
