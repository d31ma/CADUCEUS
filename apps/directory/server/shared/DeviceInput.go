import (
	"cmp"
	"crypto/ed25519"
	"encoding/base64"
	"maps"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

type DeviceInput struct{}

func (input DeviceInput) Cleaned(add DeviceAdd) DeviceAdd {
	add.Name = strings.TrimSpace(add.Name)
	return add
}

func (input DeviceInput) Check(add DeviceAdd) error {
	return cmp.Or(
		input.refusedUnless(input.deviceId(add.DeviceId), InvalidRequest{}.BadDevice("deviceId")),
		input.refusedUnless(input.countFits(utf8.RuneCountInString(add.Name), input.longestName()), InvalidRequest{}.BadDevice("name")),
		input.refusedUnless(input.publicKey(add.IdentityKey), InvalidRequest{}.BadDevice("identityKey")),
		input.refusedUnless(input.publicKey(add.AgreementKey), InvalidRequest{}.BadDevice("agreementKey")),
		input.refusedUnless(input.optionalKey(add.BackupKey), InvalidRequest{}.BadDevice("backupKey")),
	)
}

func (input DeviceInput) PersonIds(text string) ([]string, error) {
	ids := input.distinct(strings.Split(text, input.listSeparator()))
	return ids, input.refusedUnless(input.countFits(len(ids), input.mostPeople()), InvalidRequest{}.BadDevice("personIds"))
}

func (input DeviceInput) DeviceIds(given []string) ([]string, error) {
	ids := input.distinct(given)
	return ids, input.refusedUnless(input.countFits(len(ids), input.mostDevices()), InvalidRequest{}.BadDevice("deviceIds"))
}

func (input DeviceInput) SignedText(add DeviceAdd) []byte {
	return []byte(strings.Join([]string{input.textVersion(), add.DeviceId, add.IdentityKey, add.AgreementKey, add.BackupKey}, input.lineBreak()))
}

func (input DeviceInput) SignedBy(signerKey string, add DeviceAdd) bool {
	return input.verified(input.keyBytes(signerKey), input.SignedText(add), input.signatureBytes(add.Signature))
}

func (input DeviceInput) refusedUnless(valid bool, refusal error) error {
	if valid {
		return nil
	}
	return refusal
}

func (input DeviceInput) deviceId(text string) bool {
	return regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`).MatchString(text)
}

func (input DeviceInput) countFits(count int, most int) bool {
	return count >= 1 && count <= most
}

func (input DeviceInput) publicKey(text string) bool {
	return len(input.keyBytes(text)) == ed25519.PublicKeySize
}

func (input DeviceInput) optionalKey(text string) bool {
	if text == "" {
		return true
	}
	return input.publicKey(text)
}

func (input DeviceInput) keyBytes(text string) ed25519.PublicKey {
	decoded, failure := base64.RawURLEncoding.DecodeString(text)
	if failure != nil {
		return nil
	}
	return decoded
}

func (input DeviceInput) signatureBytes(text string) []byte {
	decoded, _ := base64.RawURLEncoding.DecodeString(text)
	return decoded
}

func (input DeviceInput) verified(key ed25519.PublicKey, message []byte, signature []byte) bool {
	return len(key) == ed25519.PublicKeySize && ed25519.Verify(key, message, signature)
}

func (input DeviceInput) distinct(given []string) []string {
	seen := map[string]bool{}
	for _, id := range given {
		seen[strings.TrimSpace(id)] = true
	}
	delete(seen, "")
	return slices.Sorted(maps.Keys(seen))
}

func (input DeviceInput) longestName() int {
	return 64
}

func (input DeviceInput) mostPeople() int {
	return 50
}

func (input DeviceInput) mostDevices() int {
	return 200
}

func (input DeviceInput) listSeparator() string {
	return ","
}

func (input DeviceInput) lineBreak() string {
	return "\n"
}

func (input DeviceInput) textVersion() string {
	return "mesail-device-v1"
}
