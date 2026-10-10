import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

// DeliveryFields reads and builds the loosely typed JSON records the
// delivery code passes around, with the JavaScript meanings it relied on.
type DeliveryFields struct{}

// Strings answers the text items of a JSON array (nil when it is not one).
func (DeliveryFields) Strings(value any) []string {
	list, isList := RecordValues{}.List(value)
	if !isList {
		return nil
	}
	texts := []string{}
	for _, item := range list {
		if text, isText := item.(string); isText {
			texts = append(texts, text)
		}
	}
	return texts
}

// Items answers texts as a JSON array.
func (DeliveryFields) Items(texts []string) []any {
	items := make([]any, len(texts))
	for index, text := range texts {
		items[index] = text
	}
	return items
}

// Millis answers a stored millisecond instant.
func (DeliveryFields) Millis(value any) (int64, bool) {
	number, isNumber := RecordValues{}.Number(value)
	return int64(number), isNumber
}

// Domain answers the part of an address after "@", or "".
func (DeliveryFields) Domain(address string) string {
	parts := strings.SplitN(address, "@", 3)
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

// Copy answers a shallow copy of a record.
func (DeliveryFields) Copy(record map[string]any) map[string]any {
	copied := map[string]any{}
	for key, value := range record {
		copied[key] = value
	}
	return copied
}

// Merged answers a copy of record with the fields of each patch in order,
// like the JavaScript {...record, ...patch} spread.
func (fields DeliveryFields) Merged(record map[string]any, patches ...map[string]any) map[string]any {
	merged := fields.Copy(record)
	for _, patch := range patches {
		for key, value := range patch {
			merged[key] = value
		}
	}
	return merged
}

// Serial answers the "<prefix><time in base 36>-<8 random base-36
// characters>" identifiers the JavaScript server issued with Date.now() and
// Math.random().
func (fields DeliveryFields) Serial(prefix string) string {
	return prefix + strconv.FormatInt(EpochTime{}.Now(), 36) + "-" + fields.RandomBase36(8)
}

// RandomBase36 answers size random lower-case letters and digits.
func (DeliveryFields) RandomBase36(size int) string {
	alphabet := "0123456789abcdefghijklmnopqrstuvwxyz"
	var built strings.Builder
	for built.Len() < size {
		index, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			break
		}
		built.WriteByte(alphabet[index.Int64()])
	}
	return built.String()
}

// Has reports whether a JSON object names the field (even as null), like
// Object.hasOwn.
func (DeliveryFields) Has(record map[string]any, field string) bool {
	_, has := record[field]
	return has
}

// UuidSeven answers a version 7 UUID: 48 bits of Unix milliseconds, then
// random bits, so identifiers sort by creation time.
func (DeliveryFields) UuidSeven() string {
	bytes := make([]byte, 16)
	_, _ = rand.Read(bytes)
	now := EpochTime{}.Now()
	for index := 0; index < 6; index++ {
		bytes[index] = byte(now >> (8 * (5 - index)))
	}
	bytes[6] = bytes[6]&0x0f | 0x70
	bytes[8] = bytes[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16])
}
