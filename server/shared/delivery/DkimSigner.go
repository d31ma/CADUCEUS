import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// DkimSigner adds an RFC 6376 DKIM-Signature (rsa-sha256, relaxed/relaxed)
// to outbound messages, with the key DKIM_DOMAIN, DKIM_SELECTOR and
// DKIM_PRIVATE_KEY (or DKIM_PRIVATE_KEY_FILE) configure.
type DkimSigner struct {
	Domain   string
	Selector string
	Key      *rsa.PrivateKey
}

// FromSettings answers the configured signer, or false when DKIM is not
// configured (or its key does not parse).
func (DkimSigner) FromSettings() (DkimSigner, bool) {
	settings := Settings{}
	domain, selector := settings.Value("DKIM_DOMAIN"), settings.Value("DKIM_SELECTOR")
	privateKey := settings.Value("DKIM_PRIVATE_KEY")
	if privateKey == "" && settings.Value("DKIM_PRIVATE_KEY_FILE") != "" {
		if data, err := os.ReadFile(settings.Value("DKIM_PRIVATE_KEY_FILE")); err == nil {
			privateKey = string(data)
		}
	}
	if domain == "" || selector == "" || privateKey == "" {
		return DkimSigner{}, false
	}
	key, err := DkimSigner{}.parsedKey(privateKey)
	if err != nil {
		return DkimSigner{}, false
	}
	return DkimSigner{Domain: domain, Selector: selector, Key: key}, true
}

func (DkimSigner) parsedKey(text string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(strings.ReplaceAll(text, `\n`, "\n")))
	if block == nil {
		return nil, errors.New("DKIM private key is not PEM")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, isRsa := parsed.(*rsa.PrivateKey)
	if !isRsa {
		return nil, errors.New("DKIM private key is not RSA")
	}
	return key, nil
}

// Signed answers the message with its DKIM-Signature header first.
func (signer DkimSigner) Signed(message []byte) ([]byte, error) {
	split := bytes.Index(message, []byte("\r\n\r\n"))
	if split < 0 {
		return nil, errors.New("message has no header block")
	}
	headers, body := signer.headerFields(string(message[:split+2])), message[split+4:]
	bodyHash := sha256.Sum256(signer.canonicalBody(body))
	signedNames := []string{}
	hashed := strings.Builder{}
	for _, name := range signer.signedHeaders() {
		if field, found := headers[name]; found {
			signedNames = append(signedNames, name)
			hashed.WriteString(signer.canonicalHeader(field) + "\r\n")
		}
	}
	value := "v=1; a=rsa-sha256; c=relaxed/relaxed; d=" + signer.Domain + "; s=" + signer.Selector +
		"; t=" + strconv.FormatInt(EpochTime{}.Now()/1000, 10) + "; h=" + strings.Join(signedNames, ":") +
		"; bh=" + base64.StdEncoding.EncodeToString(bodyHash[:]) + "; b="
	hashed.WriteString(signer.canonicalHeader("DKIM-Signature: " + value))
	digest := sha256.Sum256([]byte(hashed.String()))
	signature, err := rsa.SignPKCS1v15(rand.Reader, signer.Key, crypto.SHA256, digest[:])
	if err != nil {
		return nil, err
	}
	header := "DKIM-Signature: " + value + base64.StdEncoding.EncodeToString(signature) + "\r\n"
	return append([]byte(header), message...), nil
}

// headerFields answers each header field (unfolded name: value) by its
// lower-case name; the last occurrence wins, as DKIM signs bottom-up.
func (DkimSigner) headerFields(block string) map[string]string {
	fields := map[string]string{}
	lines := strings.Split(strings.TrimSuffix(block, "\r\n"), "\r\n")
	current := ""
	flush := func() {
		if colon := strings.IndexByte(current, ':'); colon > 0 {
			fields[strings.ToLower(strings.TrimSpace(current[:colon]))] = current
		}
	}
	for _, line := range lines {
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			current += "\r\n" + line
			continue
		}
		flush()
		current = line
	}
	flush()
	return fields
}

// canonicalHeader answers one header field in relaxed form.
func (DkimSigner) canonicalHeader(field string) string {
	colon := strings.IndexByte(field, ':')
	name := strings.ToLower(strings.TrimSpace(field[:colon]))
	value := strings.ReplaceAll(field[colon+1:], "\r\n", "")
	value = strings.TrimSpace(regexp.MustCompile(`[ \t]+`).ReplaceAllString(value, " "))
	return name + ":" + value
}

// canonicalBody answers the body in relaxed form.
func (DkimSigner) canonicalBody(body []byte) []byte {
	lines := strings.Split(string(body), "\r\n")
	for index, line := range lines {
		lines[index] = strings.TrimRight(regexp.MustCompile(`[ \t]+`).ReplaceAllString(line, " "), " ")
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return []byte{}
	}
	return []byte(strings.Join(lines, "\r\n") + "\r\n")
}

func (DkimSigner) signedHeaders() []string {
	return []string{"from", "sender", "reply-to", "subject", "date", "message-id", "to", "cc", "in-reply-to", "references", "mime-version", "content-type", "content-transfer-encoding"}
}
