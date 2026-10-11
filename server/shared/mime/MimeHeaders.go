import (
	"bytes"
	"mime"
	"net/mail"
	"strings"
)

// MimeHeaders is one RFC 5322 header block: each field in order, with its
// lower-case name and its unfolded raw value.
type MimeHeaders struct {
	Names  []string
	Values []string
}

// Split answers the header block and the body of a message or part: the
// headers end at the first empty line (CRLF or LF). Without an empty line
// the whole input is headers.
func (MimeHeaders) Split(raw []byte) ([]byte, []byte) {
	for index := 0; index < len(raw); index++ {
		if raw[index] != '\n' {
			continue
		}
		rest := raw[index+1:]
		if bytes.HasPrefix(rest, []byte("\r\n")) {
			return raw[:index+1], rest[2:]
		}
		if bytes.HasPrefix(rest, []byte("\n")) {
			return raw[:index+1], rest[1:]
		}
	}
	if bytes.HasPrefix(raw, []byte("\r\n")) {
		return nil, raw[2:]
	}
	if bytes.HasPrefix(raw, []byte("\n")) {
		return nil, raw[1:]
	}
	return raw, nil
}

// Parsed answers the fields of a header block, unfolding continuation lines.
func (MimeHeaders) Parsed(block []byte) MimeHeaders {
	headers := MimeHeaders{}
	lines := strings.Split(strings.ReplaceAll(string(block), "\r\n", "\n"), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		if (line[0] == ' ' || line[0] == '\t') && len(headers.Values) > 0 {
			headers.Values[len(headers.Values)-1] += " " + strings.TrimLeft(line, " \t")
			continue
		}
		name, value, found := strings.Cut(line, ":")
		if !found || strings.TrimSpace(name) == "" {
			continue
		}
		headers.Names = append(headers.Names, strings.ToLower(strings.TrimSpace(name)))
		headers.Values = append(headers.Values, strings.TrimSpace(value))
	}
	return headers
}

// Get answers the first raw value of a field, or "".
func (headers MimeHeaders) Get(name string) string {
	for index, candidate := range headers.Names {
		if candidate == name {
			return headers.Values[index]
		}
	}
	return ""
}

// Decoded answers text with its RFC 2047 encoded words decoded.
func (MimeHeaders) Decoded(value string) string {
	decoder := mime.WordDecoder{CharsetReader: MimeCharset{}.Reader}
	decoded, err := decoder.DecodeHeader(value)
	if err != nil {
		return value
	}
	return decoded
}

// Addresses answers the lower-case mailbox addresses of an address-list
// field, without display names.
func (headers MimeHeaders) Addresses(value string) []string {
	parser := mail.AddressParser{WordDecoder: &mime.WordDecoder{CharsetReader: MimeCharset{}.Reader}}
	parsed, err := parser.ParseList(value)
	if err != nil {
		return MailAddresses{}.Found(headers.Decoded(value))
	}
	addresses := []string{}
	for _, address := range parsed {
		if address.Address != "" {
			addresses = append(addresses, strings.ToLower(address.Address))
		}
	}
	return addresses
}

// MediaType answers a structured field's value (such as text/plain) in
// lower case, and its parameters with RFC 2231 and RFC 2047 values decoded.
func (headers MimeHeaders) MediaType(value string) (string, map[string]string) {
	kind, parameters, err := mime.ParseMediaType(value)
	if err != nil {
		kind = strings.ToLower(strings.TrimSpace(strings.SplitN(value, ";", 2)[0]))
		parameters = map[string]string{}
	}
	for key, parameter := range parameters {
		parameters[key] = headers.Decoded(parameter)
	}
	return kind, parameters
}

// addressFields are the fields whose values are address lists.
func (MimeHeaders) addressFields() []string {
	return []string{"from", "to", "cc", "bcc", "sender", "reply-to", "delivered-to", "return-path"}
}

// Flattened answers every field as lower-case name to text, as the server
// stored parsed headers: address fields as their comma-joined addresses,
// structured fields as their value without parameters, repeated fields
// joined with commas and every other field decoded.
func (headers MimeHeaders) Flattened() map[string]any {
	flattened := map[string]any{}
	for index, name := range headers.Names {
		value := headers.flat(name, headers.Values[index])
		if previous, seen := flattened[name]; seen {
			value = previous.(string) + "," + value
		}
		flattened[name] = value
	}
	return flattened
}

func (headers MimeHeaders) flat(name string, value string) string {
	for _, field := range headers.addressFields() {
		if field == name {
			return strings.Join(headers.Addresses(value), ", ")
		}
	}
	if name == "content-type" || name == "content-disposition" {
		kind, _ := headers.MediaType(value)
		return kind
	}
	return headers.Decoded(value)
}
