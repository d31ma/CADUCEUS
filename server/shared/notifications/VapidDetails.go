// VapidDetails identifies this server to push services: a mailto: or
// https: subject and the VAPID key pair, both as unpadded base64url.
type VapidDetails struct {
	Subject    string
	PublicKey  string
	PrivateKey string
}
