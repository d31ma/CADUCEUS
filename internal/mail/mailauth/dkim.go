package mailauth

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"hash"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// headerRow is one (possibly folded) header line, as mailauth's parseHeaders.
type headerRow struct {
	key   string // lowercased name
	cased string
	line  string // without the final CRLF, folds kept as CRLF
}

func splitMessage(raw []byte) ([]headerRow, []byte) {
	var headerPart, body []byte
	if index := bytes.Index(raw, []byte("\r\n\r\n")); index >= 0 {
		headerPart, body = raw[:index], raw[index+4:]
	} else if index := bytes.Index(raw, []byte("\n\n")); index >= 0 {
		headerPart, body = raw[:index], raw[index+2:]
	} else {
		headerPart = raw
	}
	text := strings.TrimRight(string(headerPart), "\r\n")
	lines := regexp.MustCompile(`\r?\n`).Split(text, -1)
	var groups [][]string
	for _, line := range lines {
		if len(groups) > 0 && (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")) {
			groups[len(groups)-1] = append(groups[len(groups)-1], line)
			continue
		}
		groups = append(groups, []string{line})
	}
	rows := make([]headerRow, 0, len(groups))
	for _, group := range groups {
		line := strings.Join(group, "\r\n")
		row := headerRow{line: line}
		if colon := strings.Index(line, ":"); colon > 0 {
			row.cased = strings.TrimSpace(line[:colon])
			row.key = strings.ToLower(row.cased)
		} else if colon < 0 && line != "" {
			row.cased = strings.TrimSpace(line)
			row.key = strings.ToLower(row.cased)
		}
		rows = append(rows, row)
	}
	return rows, body
}

// tags parses a DKIM tag list (signature or key record).
func tags(value string) map[string]string {
	result := map[string]string{}
	for _, part := range strings.Split(value, ";") {
		equals := strings.Index(part, "=")
		if equals < 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(part[:equals]))
		tagValue := strings.TrimSpace(spaceRun.ReplaceAllString(part[equals+1:], " "))
		switch key {
		case "b", "bh", "p", "h":
			tagValue = spaceRun.ReplaceAllString(tagValue, "")
		}
		if key != "" {
			result[key] = tagValue
		}
	}
	return result
}

var (
	relaxedKey = regexp.MustCompile(`^([^:]*):\s*`)
	unfold     = regexp.MustCompile(`\r?\n`)
	stripB     = regexp.MustCompile(`([;:\s]+b=)[^;]+`)
	wspRun     = regexp.MustCompile(`[ \t]+`)
)

func relaxedLine(line string) string {
	line = unfold.ReplaceAllString(line, "")
	line = relaxedKey.ReplaceAllStringFunc(line, func(match string) string {
		return strings.ToLower(strings.TrimSpace(match[:strings.Index(match, ":")])) + ":"
	})
	return strings.TrimSpace(spaceRun.ReplaceAllString(line, " "))
}

func stripSignature(line string) string {
	location := stripB.FindStringSubmatchIndex(line)
	if location == nil {
		return line
	}
	return line[:location[3]] + line[location[1]:]
}

// canonicalBody answers the RFC 6376 body canonicalization. Like
// mailauth's hashers it works on the bytes as received: "simple" drops every
// trailing CR and LF and ends the body with one CRLF.
func canonicalBody(body []byte, relaxed bool) []byte {
	if !relaxed {
		return append(bytes.TrimRight(append([]byte{}, body...), "\r\n"), '\r', '\n')
	}
	lines := strings.Split(string(body), "\r\n")
	for index, line := range lines {
		lines[index] = strings.TrimRight(wspRun.ReplaceAllString(line, " "), " ")
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return nil
	}
	return []byte(strings.Join(lines, "\r\n") + "\r\n")
}

// DKIMResult is one signature check.
type DKIMResult struct {
	SigningDomain string
	Selector      string
	Status        Status
	Aligned       string
	UnderSized    int
	Info          string
}

type keyError struct {
	code string
}

func (err *keyError) Error() string { return err.code }

func publicKey(ctx context.Context, resolver Resolver, name string) (crypto.PublicKey, error) {
	records, err := resolver.LookupTXT(ctx, name)
	if err != nil {
		return nil, err
	}
	record := ""
	if len(records) > 0 {
		record = spaceRun.ReplaceAllString(records[0], "")
	}
	if record == "" {
		return nil, &keyError{"EINVALIDVAL"}
	}
	fields := tags(record)
	encoded := fields["p"]
	if encoded == "" {
		return nil, &keyError{"EINVALIDVAL"}
	}
	if version, ok := fields["v"]; ok && strings.ToLower(strings.TrimSpace(version)) != "dkim1" {
		return nil, &keyError{"EINVALIDVER"}
	}
	der, err := base64.RawStdEncoding.DecodeString(strings.TrimRight(encoded, "="))
	if err != nil {
		return nil, &keyError{"EINVALIDVAL"}
	}
	var key crypto.PublicKey
	keyType := ""
	if len(der) == ed25519.PublicKeySize {
		key, keyType = ed25519.PublicKey(der), "ed25519"
	} else {
		parsed, err := x509.ParsePKIXPublicKey(der)
		if err != nil {
			return nil, &keyError{"EINVALIDTYPE"}
		}
		switch typed := parsed.(type) {
		case *rsa.PublicKey:
			key, keyType = typed, "rsa"
		case ed25519.PublicKey:
			key, keyType = typed, "ed25519"
		default:
			return nil, &keyError{"EINVALIDTYPE"}
		}
	}
	if declared, ok := fields["k"]; ok && strings.ToLower(declared) != keyType {
		return nil, &keyError{"EINVALIDTYPE"}
	}
	if rsaKey, ok := key.(*rsa.PublicKey); ok && rsaKey.N.BitLen() < 1024 {
		return nil, &keyError{"ESHORTKEY"}
	}
	return key, nil
}

func headerFromAddresses(rows []headerRow) []string {
	var addresses []string
	for _, row := range rows {
		if row.key != "from" {
			continue
		}
		value := row.line[strings.Index(row.line, ":")+1:]
		value = strings.TrimSpace(unfold.ReplaceAllString(value, ""))
		if list, err := (&mail.AddressParser{}).ParseList(value); err == nil {
			for _, address := range list {
				addresses = append(addresses, address.Address)
			}
			continue
		}
		addresses = append(addresses, looseAddresses(value)...)
	}
	return addresses
}

var looseAddressPattern = regexp.MustCompile(`[^\s<>,;:"()\[\]]+@[^\s<>,;:"()\[\]]+`)

func looseAddresses(value string) []string { return looseAddressPattern.FindAllString(value, -1) }

func newHash(name string) (hash.Hash, crypto.Hash) {
	if name == "sha1" {
		return sha1.New(), crypto.SHA1
	}
	return sha256.New(), crypto.SHA256
}

// verifyDKIM checks every DKIM-Signature as mailauth's DkimVerifier did.
func verifyDKIM(ctx context.Context, resolver Resolver, raw []byte, now time.Time) ([]DKIMResult, []string) {
	rows, body := splitMessage(raw)
	headerFrom := headerFromAddresses(rows)
	var results []DKIMResult
	for _, row := range rows {
		if row.key != "dkim-signature" {
			continue
		}
		fields := tags(row.line[strings.Index(row.line, ":")+1:])
		algorithm := fields["a"]
		pieces := strings.Split(algorithm, "-")
		signAlgo := strings.ToLower(strings.TrimSpace(pieces[0]))
		hashAlgo := strings.ToLower(strings.TrimSpace(pieces[len(pieces)-1]))
		canonicalization := strings.Split(fields["c"], "/")
		headerCanon := strings.ToLower(strings.TrimSpace(canonicalization[0]))
		if headerCanon == "" {
			headerCanon = "simple"
		}
		bodyCanon := "simple"
		if len(canonicalization) > 1 {
			bodyCanon = strings.ToLower(strings.TrimSpace(canonicalization[1]))
		}
		domain, selector := fields["d"], fields["s"]
		if (signAlgo != "rsa" && signAlgo != "ed25519") || (hashAlgo != "sha256" && hashAlgo != "sha1") ||
			(headerCanon != "relaxed" && headerCanon != "simple") || (bodyCanon != "relaxed" && bodyCanon != "simple") ||
			domain == "" || selector == "" {
			continue
		}

		canonical := canonicalBody(body, bodyCanon == "relaxed")
		total := len(canonical)
		hashed := canonical
		limit := -1
		if value, ok := fields["l"]; ok && value != "" {
			if parsed, err := strconv.Atoi(value); err == nil && parsed >= 0 {
				limit = parsed
				if parsed < len(hashed) {
					hashed = hashed[:parsed]
				}
			}
		}
		bodyHasher, _ := newHash(hashAlgo)
		bodyHasher.Write(hashed)
		bodyHash := base64.StdEncoding.EncodeToString(bodyHasher.Sum(nil))

		var signed strings.Builder
		used := make([]bool, len(rows))
		for _, name := range strings.Split(fields["h"], ":") {
			name = strings.ToLower(strings.TrimSpace(name))
			if name == "" {
				continue
			}
			for index := len(rows) - 1; index >= 0; index-- {
				if !used[index] && rows[index].key == name {
					used[index] = true
					if headerCanon == "relaxed" {
						signed.WriteString(relaxedLine(rows[index].line) + "\r\n")
					} else {
						signed.WriteString(rows[index].line + "\r\n")
					}
					break
				}
			}
		}
		if headerCanon == "relaxed" {
			signed.WriteString(stripSignature(relaxedLine(row.line)))
		} else {
			signed.WriteString(stripSignature(row.line))
		}

		signature := fields["b"]
		short := signature
		if len(short) > 8 {
			short = short[:8]
		}
		status := Status{Result: "neutral", Header: []Prop{{"i", "@" + domain}, {"s", selector}, {"a", algorithm}, {"b", short}}}
		result := DKIMResult{SigningDomain: domain, Selector: selector}
		if len(headerFrom) > 0 {
			fromDomain := headerFrom[0][strings.LastIndex(headerFrom[0], "@")+1:]
			if aligned, ok := alignment(fromDomain, []alignedDomain{{domain: domain}}, false); ok {
				result.Aligned = aligned.domain
			}
		}

		var timestamp, expiration *time.Time
		if value, err := strconv.ParseInt(fields["t"], 10, 64); err == nil {
			stamp := time.Unix(value, 0)
			timestamp = &stamp
		}
		if value, err := strconv.ParseInt(fields["x"], 10, 64); err == nil {
			stamp := time.Unix(value, 0)
			expiration = &stamp
		}

		if fields["bh"] != bodyHash {
			status.Comment = "body hash did not verify"
		} else if key, err := publicKey(ctx, resolver, selector+"._domainkey."+domain); err != nil {
			var keyErr *keyError
			switch {
			case IsNotFound(err):
				status.Comment = "no key"
			case errors.As(err, &keyErr) && keyErr.code == "EINVALIDVER":
				status.Comment = "unknown key version"
			case errors.As(err, &keyErr) && keyErr.code == "EINVALIDTYPE":
				status.Comment = "unknown key type"
			case errors.As(err, &keyErr) && keyErr.code == "EINVALIDVAL":
				status.Comment = "invalid public key"
			case errors.As(err, &keyErr) && keyErr.code == "ESHORTKEY":
				status.Result = "policy"
				status.Policy = []Prop{{"dkim-rules", "weak-key"}}
			default:
				status.Result, status.Comment = "temperror", "DNS failure: "+code(err)
			}
		} else {
			sig, decodeErr := base64.StdEncoding.DecodeString(signature)
			if decodeErr != nil {
				sig, decodeErr = base64.RawStdEncoding.DecodeString(strings.TrimRight(signature, "="))
			}
			verified := false
			var verifyErr error
			switch typed := key.(type) {
			case *rsa.PublicKey:
				if signAlgo != "rsa" {
					verifyErr = errors.New("key type does not match the signature algorithm")
					break
				}
				hasher, kind := newHash(hashAlgo)
				hasher.Write([]byte(signed.String()))
				verified = decodeErr == nil && rsa.VerifyPKCS1v15(typed, kind, hasher.Sum(nil), sig) == nil
			case ed25519.PublicKey:
				if signAlgo != "ed25519" {
					verifyErr = errors.New("key type does not match the signature algorithm")
					break
				}
				digest := sha256.Sum256([]byte(signed.String()))
				verified = decodeErr == nil && ed25519.Verify(typed, digest[:], sig)
			}
			switch {
			case verifyErr != nil:
				status.Comment = verifyErr.Error()
			case verified:
				status.Result = "pass"
				if expiration != nil && timestamp != nil && expiration.Before(*timestamp) {
					status.Result, status.Comment = "neutral", "invalid expiration"
				}
				if expiration != nil && expiration.Before(now) {
					status.Result, status.Comment = "neutral", "expired"
				}
			default:
				status.Result, status.Comment = "fail", "bad signature"
			}
		}
		// mailauth stops counting the relaxed body once the l= limit is
		// reached, which leaves the final CRLF out of its total.
		if limit >= 0 && bodyCanon == "relaxed" && limit < total-2 {
			total -= 2
		}
		if limit >= 0 && total > len(hashed) {
			status.UnderSized = total - len(hashed)
			result.UnderSized = status.UnderSized
		}
		result.Status = status
		result.Info = status.Format("dkim")
		results = append(results, result)
	}
	if len(results) == 0 {
		status := Status{Result: "none", Comment: "message not signed"}
		results = append(results, DKIMResult{Status: status, Info: status.Format("dkim")})
	}
	return results, headerFrom
}
