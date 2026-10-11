import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// WebPush sends one payload to one push subscription the way the web-push
// module did: the payload encrypted with aes128gcm (RFC 8188 and RFC 8291),
// a VAPID ES256 authorization (RFC 8292), a four-week TTL and normal urgency.
// A push service answer outside 2xx is a failure.
type WebPush struct{}

// Send delivers payload to the subscription {endpoint, keys: {p256dh, auth}}.
func (push WebPush) Send(subscription map[string]any, payload []byte, vapid WebPushVapid) error {
	endpoint, _ := subscription["endpoint"].(string)
	if endpoint == "" {
		return errors.New("You must pass in a subscription with at least an endpoint.")
	}
	keys, _ := subscription["keys"].(map[string]any)
	receiverKey, _ := keys["p256dh"].(string)
	authSecret, _ := keys["auth"].(string)
	if receiverKey == "" || authSecret == "" {
		return errors.New("To send a message with a payload, the subscription must have 'auth' and 'p256dh' keys.")
	}
	body, err := push.encrypted(receiverKey, authSecret, payload)
	if err != nil {
		return err
	}
	request, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("TTL", strconv.Itoa(push.timeToLive()))
	request.Header.Set("Content-Type", "application/octet-stream")
	request.Header.Set("Content-Encoding", "aes128gcm")
	request.Header.Set("Urgency", "normal")
	// A legacy GCM endpoint takes a GCM key, which CADUCEUS never had.
	if !strings.HasPrefix(endpoint, "https://android.googleapis.com/gcm/send") {
		authorization, err := push.authorization(request.URL, vapid)
		if err != nil {
			return err
		}
		request.Header.Set("Authorization", authorization)
	}
	return push.deliver(request)
}

func (WebPush) timeToLive() int { return 2419200 }

func (push WebPush) deliver(request *http.Request) error {
	response, err := (&http.Client{Timeout: push.timeout()}).Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return fmt.Errorf("Received unexpected response code %d", response.StatusCode)
	}
	return nil
}

func (WebPush) timeout() time.Duration { return 30 * time.Second }

// authorization answers "vapid t=<JWT>, k=<public key>" for the origin of
// the push service, valid for twelve hours.
func (push WebPush) authorization(endpoint *url.URL, vapid WebPushVapid) (string, error) {
	if err := push.validSubject(vapid.Subject); err != nil {
		return "", err
	}
	publicKey, err := push.decoded(vapid.PublicKey)
	if err != nil || len(publicKey) != 65 {
		return "", errors.New("Vapid public key should be 65 bytes long when decoded.")
	}
	privateKey, err := push.decoded(vapid.PrivateKey)
	if err != nil || len(privateKey) != 32 {
		return "", errors.New("Vapid private key should be 32 bytes long when decoded.")
	}
	signer, err := ecdsa.ParseRawPrivateKey(elliptic.P256(), privateKey)
	if err != nil {
		return "", err
	}
	audience := endpoint.Scheme + "://" + strings.ToLower(endpoint.Host)
	expires := time.Now().Unix() + push.vapidLifetime()
	audienceText, _ := json.Marshal(audience)
	subjectText, _ := json.Marshal(vapid.Subject)
	claims := fmt.Sprintf(`{"aud":%s,"exp":%d,"sub":%s}`, audienceText, expires, subjectText)
	encoding := base64.RawURLEncoding
	signed := encoding.EncodeToString([]byte(`{"typ":"JWT","alg":"ES256"}`)) + "." + encoding.EncodeToString([]byte(claims))
	digest := sha256.Sum256([]byte(signed))
	r, s, err := ecdsa.Sign(rand.Reader, signer, digest[:])
	if err != nil {
		return "", err
	}
	signature := make([]byte, 64)
	r.FillBytes(signature[:32])
	s.FillBytes(signature[32:])
	return "vapid t=" + signed + "." + encoding.EncodeToString(signature) + ", k=" + vapid.PublicKey, nil
}

// decoded answers base64url text (standard base64 and padding are also
// accepted, as Node's decoder accepted them).
func (WebPush) decoded(text string) ([]byte, error) {
	normal := strings.NewReplacer("+", "-", "/", "_").Replace(strings.TrimRight(text, "="))
	return base64.RawURLEncoding.DecodeString(normal)
}

func (WebPush) vapidLifetime() int64 { return 12 * 60 * 60 }

func (WebPush) validSubject(subject string) error {
	if strings.HasPrefix(subject, "mailto:") {
		return nil
	}
	parsed, err := url.Parse(subject)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return errors.New("Vapid subject is not a url or mailto url. " + subject)
	}
	return nil
}

// encrypted answers the aes128gcm body: the header (salt, record size and
// the sender's public key as key id) and the encrypted records.
func (push WebPush) encrypted(receiverKey string, authSecret string, payload []byte) ([]byte, error) {
	receiver, err := push.decoded(receiverKey)
	if err != nil || len(receiver) != 65 {
		return nil, errors.New("The subscription p256dh value should be 65 bytes long.")
	}
	auth, err := push.decoded(authSecret)
	if err != nil || len(auth) < 16 {
		return nil, errors.New("The subscription auth key should be at least 16 bytes long")
	}
	key, nonce, header, err := push.keys(receiver, auth)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	sealer, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return push.records(sealer, nonce, header, payload), nil
}

// keys derives the content key and nonce from a fresh sender key pair and
// salt (RFC 8291 section 3.4) and answers them with the body header.
func (push WebPush) keys(receiver []byte, auth []byte) ([]byte, []byte, []byte, error) {
	receiverKey, err := ecdh.P256().NewPublicKey(receiver)
	if err != nil {
		return nil, nil, nil, err
	}
	sender, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, nil, err
	}
	shared, err := sender.ECDH(receiverKey)
	if err != nil {
		return nil, nil, nil, err
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, nil, nil, err
	}
	senderPublic := sender.PublicKey().Bytes()
	info := append(append([]byte("WebPush: info\x00"), receiver...), senderPublic...)
	secret, err := hkdf.Key(sha256.New, shared, auth, string(info), 32)
	if err != nil {
		return nil, nil, nil, err
	}
	pseudorandom, err := hkdf.Extract(sha256.New, secret, salt)
	if err != nil {
		return nil, nil, nil, err
	}
	key, err := hkdf.Expand(sha256.New, pseudorandom, "Content-Encoding: aes128gcm\x00", 16)
	if err != nil {
		return nil, nil, nil, err
	}
	nonce, err := hkdf.Expand(sha256.New, pseudorandom, "Content-Encoding: nonce\x00", 12)
	if err != nil {
		return nil, nil, nil, err
	}
	header := binary.BigEndian.AppendUint32(append([]byte{}, salt...), uint32(push.recordSize()))
	header = append(append(header, byte(len(senderPublic))), senderPublic...)
	return key, nonce, header, nil
}

func (WebPush) recordSize() int { return 4096 }

// records splits the payload into records of recordSize - 17 bytes, each
// ending in the delimiter 1, or 2 for the last record, sealed with the nonce
// of its sequence number.
func (push WebPush) records(sealer cipher.AEAD, nonce []byte, header []byte, payload []byte) []byte {
	body := header
	chunk := push.recordSize() - sealer.Overhead() - 1
	for start, sequence := 0, uint64(0); ; sequence++ {
		end := min(start+chunk, len(payload))
		last := start+chunk >= len(payload)
		delimiter := byte(1)
		if last {
			delimiter = 2
		}
		plain := append(append([]byte{}, payload[start:end]...), delimiter)
		body = sealer.Seal(body, push.recordNonce(nonce, sequence), plain, nil)
		if last {
			return body
		}
		start = end
	}
}

func (WebPush) recordNonce(nonce []byte, sequence uint64) []byte {
	counted := append([]byte{}, nonce...)
	tail := binary.BigEndian.Uint64(append([]byte{0, 0}, counted[6:]...)) ^ sequence
	var written [8]byte
	binary.BigEndian.PutUint64(written[:], tail)
	copy(counted[6:], written[2:])
	return counted
}
