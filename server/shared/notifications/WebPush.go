import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// WebPush sends one Web Push message as the web-push package did: the
// payload encrypted with aes128gcm (RFC 8291, one 4096-byte record size),
// a VAPID ES256 JWT (RFC 8292) valid for 12 hours, TTL four weeks and
// normal urgency.
type WebPush struct{}

// GenerateKeys answers a new VAPID key pair: the uncompressed P-256 public
// point and the 32-byte private scalar, as unpadded base64url.
func (WebPush) GenerateKeys() (string, string) {
	key, _ := ecdh.P256().GenerateKey(rand.Reader)
	encoding := base64.RawURLEncoding
	return encoding.EncodeToString(key.PublicKey().Bytes()), encoding.EncodeToString(key.Bytes())
}

// Send delivers payload to a subscription ({endpoint, keys: {p256dh, auth}})
// and fails unless the push service answers 2xx.
func (push WebPush) Send(subscription map[string]any, payload string, vapid VapidDetails) error {
	endpoint, _ := subscription["endpoint"].(string)
	if endpoint == "" {
		return errors.New("The subscription endpoint must be a string with a valid URL.")
	}
	keys, _ := subscription["keys"].(map[string]any)
	userKey, userAuth := push.decoded(keys["p256dh"]), push.decoded(keys["auth"])
	if userKey == nil || userAuth == nil {
		return errors.New("To send a message with a payload, the subscription must have 'auth' and 'p256dh' keys.")
	}
	body, err := push.Encrypted(userKey, userAuth, []byte(payload))
	if err != nil {
		return err
	}
	request, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("TTL", "2419200")
	request.Header.Set("Content-Length", strconv.Itoa(len(body)))
	request.Header.Set("Content-Type", "application/octet-stream")
	request.Header.Set("Content-Encoding", "aes128gcm")
	if !strings.HasPrefix(endpoint, "https://android.googleapis.com/gcm/send") {
		authorization, err := push.authorization(endpoint, vapid)
		if err != nil {
			return err
		}
		request.Header.Set("Authorization", authorization)
	}
	request.Header.Set("Urgency", "normal")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return fmt.Errorf("Received unexpected response code %d", response.StatusCode)
	}
	return nil
}

// authorization answers the VAPID Authorization header for the endpoint's
// origin.
func (push WebPush) authorization(endpoint string, vapid VapidDetails) (string, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("VAPID audience is not a url. " + endpoint)
	}
	if !strings.HasPrefix(vapid.Subject, "mailto:") && !strings.HasPrefix(vapid.Subject, "https:") {
		return "", errors.New("Vapid subject is not a url or mailto url. " + vapid.Subject)
	}
	public := push.decoded(vapid.PublicKey)
	private := push.decoded(vapid.PrivateKey)
	if len(public) != 65 {
		return "", errors.New("Vapid public key should be 65 bytes long when decoded.")
	}
	if len(private) != 32 {
		return "", errors.New("Vapid private key should be 32 bytes long when decoded.")
	}
	token, err := push.token(parsed.Scheme+"://"+parsed.Host, vapid.Subject, private)
	if err != nil {
		return "", err
	}
	return "vapid t=" + token + ", k=" + vapid.PublicKey, nil
}

// token answers the ES256 JWT {aud, exp: now + 12 hours, sub}.
func (WebPush) token(audience string, subject string, privateKey []byte) (string, error) {
	key, err := ecdsa.ParseRawPrivateKey(elliptic.P256(), privateKey)
	if err != nil {
		return "", err
	}
	encoding := base64.RawURLEncoding
	header := encoding.EncodeToString([]byte(`{"typ":"JWT","alg":"ES256"}`))
	claims, _ := json.Marshal(struct {
		Aud string `json:"aud"`
		Exp int64  `json:"exp"`
		Sub string `json:"sub"`
	}{audience, time.Now().Unix() + 43_200, subject})
	signingInput := header + "." + encoding.EncodeToString(claims)
	digest := sha256.Sum256([]byte(signingInput))
	r, s, err := ecdsa.Sign(rand.Reader, key, digest[:])
	if err != nil {
		return "", err
	}
	signature := make([]byte, 64)
	r.FillBytes(signature[:32])
	s.FillBytes(signature[32:])
	return signingInput + "." + encoding.EncodeToString(signature), nil
}

// Encrypted answers the aes128gcm body for a user agent's public key and
// auth secret: salt, record size, the sender's public key, then records.
func (push WebPush) Encrypted(userPublicKey []byte, authSecret []byte, plaintext []byte) ([]byte, error) {
	curve := ecdh.P256()
	userKey, err := curve.NewPublicKey(userPublicKey)
	if err != nil {
		return nil, err
	}
	local, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	shared, err := local.ECDH(userKey)
	if err != nil {
		return nil, err
	}
	localPublic := local.PublicKey().Bytes()
	info := append(append([]byte("WebPush: info\x00"), userPublicKey...), localPublic...)
	secret := push.expand(push.extract(authSecret, shared), info, 32)
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	key := push.extract(salt, secret)
	contentKey := push.expand(key, []byte("Content-Encoding: aes128gcm\x00"), 16)
	nonce := push.expand(key, []byte("Content-Encoding: nonce\x00"), 12)
	block, err := aes.NewCipher(contentKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	recordSize := 4096
	header := make([]byte, 21)
	copy(header, salt)
	binary.BigEndian.PutUint32(header[16:], uint32(recordSize))
	header[20] = byte(len(localPublic))
	body := append(header, localPublic...)
	chunk := recordSize - 16 - 1
	for index, start := 0, 0; ; index++ {
		end := min(start+chunk, len(plaintext))
		record := append(append([]byte{}, plaintext[start:end]...), 1)
		last := end >= len(plaintext)
		if last {
			record[len(record)-1] = 2
		}
		body = gcm.Seal(body, push.recordNonce(nonce, index), record, nil)
		if last {
			return body, nil
		}
		start = end
	}
}

func (WebPush) recordNonce(base []byte, index int) []byte {
	nonce := append([]byte{}, base...)
	sequence := uint64(index)
	for position := 0; position < 6; position++ {
		nonce[len(nonce)-1-position] ^= byte(sequence >> (8 * position))
	}
	return nonce
}

// extract is HKDF-Extract with SHA-256.
func (WebPush) extract(salt []byte, secret []byte) []byte {
	mac := hmac.New(sha256.New, salt)
	mac.Write(secret)
	return mac.Sum(nil)
}

// expand is HKDF-Expand with SHA-256 for at most one block.
func (WebPush) expand(key []byte, info []byte, size int) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(info)
	mac.Write([]byte{1})
	return mac.Sum(nil)[:size]
}

// decoded answers base64url (or base64) text as bytes, or nil.
func (WebPush) decoded(value any) []byte {
	text, _ := value.(string)
	if text == "" {
		return nil
	}
	normalized := strings.TrimRight(strings.NewReplacer("+", "-", "/", "_").Replace(text), "=")
	decoded, err := base64.RawURLEncoding.DecodeString(normalized)
	if err != nil {
		return nil
	}
	return decoded
}
