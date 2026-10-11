// WebPushVapid holds the application server's VAPID identity: the
// base64url P-256 public key (65 bytes) and private key (32 bytes), and the
// contact subject (a mailto: or https: URL).
type WebPushVapid struct {
	PublicKey  string
	PrivateKey string
	Subject    string
}
