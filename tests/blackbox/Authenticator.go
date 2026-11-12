//go:build blackbox

package blackbox

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"os"
	"testing"
)

type Authenticator struct {
	key          *ecdsa.PrivateKey
	credentialId []byte
	userHandle   []byte
	relyingParty string
	origin       string
	signCount    uint32
}

type creationOptions struct {
	Challenge string `json:"challenge"`
	Rp        struct {
		Id string `json:"id"`
	} `json:"rp"`
	User struct {
		Id   string `json:"id"`
		Name string `json:"name"`
	} `json:"user"`
	ExcludeCredentials     []credentialReference `json:"excludeCredentials"`
	AuthenticatorSelection struct {
		ResidentKey      string `json:"residentKey"`
		UserVerification string `json:"userVerification"`
	} `json:"authenticatorSelection"`
}

type requestOptions struct {
	Challenge        string                `json:"challenge"`
	RpId             string                `json:"rpId"`
	UserVerification string                `json:"userVerification"`
	AllowCredentials []credentialReference `json:"allowCredentials"`
}

type credentialReference struct {
	Id   string `json:"id"`
	Type string `json:"type"`
}

const (
	flagUserPresent  = 0x01
	flagUserVerified = 0x04
	flagAttested     = 0x40
)

func encode64(raw []byte) string {
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decode64(t *testing.T, text string) []byte {
	t.Helper()
	raw, err := base64.RawURLEncoding.DecodeString(text)
	if err != nil {
		t.Fatalf("not base64url: %q: %v", text, err)
	}
	return raw
}

func parseCreationOptions(t *testing.T, optionsJson string) creationOptions {
	t.Helper()
	var options creationOptions
	if err := json.Unmarshal([]byte(optionsJson), &options); err != nil {
		t.Fatalf("optionsJson is not WebAuthn creation options JSON: %v: %s", err, optionsJson)
	}
	return options
}

func parseRequestOptions(t *testing.T, optionsJson string) requestOptions {
	t.Helper()
	var options requestOptions
	if err := json.Unmarshal([]byte(optionsJson), &options); err != nil {
		t.Fatalf("optionsJson is not WebAuthn request options JSON: %v: %s", err, optionsJson)
	}
	return options
}

func NewAuthenticator(t *testing.T) *Authenticator {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	credentialId := make([]byte, 32)
	if _, err := rand.Read(credentialId); err != nil {
		t.Fatal(err)
	}
	return &Authenticator{key: key, credentialId: credentialId}
}

func (authenticator *Authenticator) CredentialId() string {
	return encode64(authenticator.credentialId)
}
func (authenticator *Authenticator) Create(t *testing.T, optionsJson string) string {
	t.Helper()
	options := parseCreationOptions(t, optionsJson)
	authenticator.relyingParty = options.Rp.Id
	authenticator.origin = passkeyOrigin(options.Rp.Id)
	authenticator.userHandle = decode64(t, options.User.Id)
	clientData := authenticator.clientData(t, "webauthn.create", options.Challenge)
	authData := authenticator.authData(flagUserPresent|flagUserVerified|flagAttested, authenticator.attestedCredential(t))
	publicKey, err := x509.MarshalPKIXPublicKey(&authenticator.key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	credential := map[string]any{
		"id":    authenticator.CredentialId(),
		"rawId": authenticator.CredentialId(),
		"type":  "public-key",
		"response": map[string]any{
			"clientDataJSON":     encode64(clientData),
			"attestationObject":  encode64(attestationObject(authData)),
			"authenticatorData":  encode64(authData),
			"publicKey":          encode64(publicKey),
			"publicKeyAlgorithm": -7,
			"transports":         []string{"internal", "hybrid"},
		},
		"authenticatorAttachment": "platform",
		"clientExtensionResults":  map[string]any{},
	}
	return marshal(t, credential)
}
func (authenticator *Authenticator) Assert(t *testing.T, optionsJson string) string {
	t.Helper()
	return authenticator.assertWith(t, optionsJson, authenticator.key, authenticator.userHandle)
}
func (authenticator *Authenticator) AssertWrongSignature(t *testing.T, optionsJson string) string {
	t.Helper()
	other := NewAuthenticator(t)
	return authenticator.assertWith(t, optionsJson, other.key, authenticator.userHandle)
}
func (authenticator *Authenticator) AssertAs(t *testing.T, optionsJson string, personId string) string {
	t.Helper()
	return authenticator.assertWith(t, optionsJson, authenticator.key, []byte(personId))
}

func (authenticator *Authenticator) assertWith(t *testing.T, optionsJson string, key *ecdsa.PrivateKey, userHandle []byte) string {
	t.Helper()
	options := parseRequestOptions(t, optionsJson)
	authenticator.signCount++
	clientData := authenticator.clientData(t, "webauthn.get", options.Challenge)
	authData := authenticator.authData(flagUserPresent|flagUserVerified, nil)
	clientHash := sha256.Sum256(clientData)
	signed := append(append([]byte{}, authData...), clientHash[:]...)
	digest := sha256.Sum256(signed)
	signature, err := ecdsa.SignASN1(rand.Reader, key, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	credential := map[string]any{
		"id":    authenticator.CredentialId(),
		"rawId": authenticator.CredentialId(),
		"type":  "public-key",
		"response": map[string]any{
			"clientDataJSON":    encode64(clientData),
			"authenticatorData": encode64(authData),
			"signature":         encode64(signature),
			"userHandle":        encode64(userHandle),
		},
		"authenticatorAttachment": "platform",
		"clientExtensionResults":  map[string]any{},
	}
	return marshal(t, credential)
}

func (authenticator *Authenticator) clientData(t *testing.T, kind string, challenge string) []byte {
	t.Helper()
	data := map[string]any{"type": kind, "challenge": challenge, "origin": authenticator.origin, "crossOrigin": false}
	return []byte(marshal(t, data))
}

func (authenticator *Authenticator) authData(flags byte, attested []byte) []byte {
	relyingPartyHash := sha256.Sum256([]byte(authenticator.relyingParty))
	data := append([]byte{}, relyingPartyHash[:]...)
	data = append(data, flags)
	data = binary.BigEndian.AppendUint32(data, authenticator.signCount)
	return append(data, attested...)
}

func (authenticator *Authenticator) attestedCredential(t *testing.T) []byte {
	t.Helper()
	data := make([]byte, 16)
	data = binary.BigEndian.AppendUint16(data, uint16(len(authenticator.credentialId)))
	data = append(data, authenticator.credentialId...)
	return append(data, authenticator.coseKey(t)...)
}

func (authenticator *Authenticator) coseKey(t *testing.T) []byte {
	t.Helper()
	point, err := authenticator.key.PublicKey.ECDH()
	if err != nil {
		t.Fatal(err)
	}
	raw := point.Bytes()
	writer := &CborWriter{}
	writer.MapHeader(5)
	writer.Integer(1)
	writer.Integer(2)
	writer.Integer(3)
	writer.Integer(-7)
	writer.Integer(-1)
	writer.Integer(1)
	writer.Integer(-2)
	writer.ByteString(raw[1:33])
	writer.Integer(-3)
	writer.ByteString(raw[33:65])
	return writer.Bytes()
}

func attestationObject(authData []byte) []byte {
	writer := &CborWriter{}
	writer.MapHeader(3)
	writer.Text("fmt")
	writer.Text("none")
	writer.Text("attStmt")
	writer.MapHeader(0)
	writer.Text("authData")
	writer.ByteString(authData)
	return writer.Bytes()
}

func passkeyOrigin(relyingParty string) string {
	origin := os.Getenv("MESAIL_PASSKEY_ORIGIN")
	if origin != "" {
		return origin
	}
	return "https://" + relyingParty
}

func marshal(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
