package dkim

import (
	"bytes"
	"crypto"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

type testField struct {
	name string
	raw  string
}

type testSignature struct {
	tags  map[string]string
	field testField
}

func testSplitMessage(message []byte) ([]testField, []byte, error) {
	index := bytes.Index(message, []byte("\r\n\r\n"))
	if index < 0 {
		return nil, nil, errors.New("no header end")
	}
	head := string(message[:index])
	body := message[index+4:]
	var fields []testField
	for _, line := range strings.Split(head, "\r\n") {
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			fields[len(fields)-1].raw += "\r\n" + line
			continue
		}
		name, _, found := strings.Cut(line, ":")
		if !found {
			return nil, nil, fmt.Errorf("bad header line %q", line)
		}
		fields = append(fields, testField{name: name, raw: line})
	}
	return fields, body, nil
}

func testCompressSpace(text string) string {
	var builder strings.Builder
	inSpace := false
	for _, character := range text {
		if character == ' ' || character == '\t' {
			inSpace = true
			continue
		}
		if inSpace {
			builder.WriteByte(' ')
			inSpace = false
		}
		builder.WriteRune(character)
	}
	if inSpace {
		builder.WriteByte(' ')
	}
	return builder.String()
}

func testRelaxedHeader(raw string) string {
	name, value, _ := strings.Cut(raw, ":")
	value = strings.ReplaceAll(value, "\r\n", "")
	value = testCompressSpace(value)
	value = strings.Trim(value, " ")
	return strings.ToLower(strings.TrimRight(name, " \t")) + ":" + value
}

func testRelaxedBody(body []byte) []byte {
	lines := strings.Split(string(body), "\r\n")
	for index := range lines {
		lines[index] = strings.TrimRight(testCompressSpace(lines[index]), " ")
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return nil
	}
	return []byte(strings.Join(lines, "\r\n") + "\r\n")
}

func testParseSignature(fields []testField) (testSignature, error) {
	for _, field := range fields {
		if strings.EqualFold(field.name, "DKIM-Signature") {
			_, value, _ := strings.Cut(field.raw, ":")
			tags := map[string]string{}
			for _, part := range strings.Split(value, ";") {
				tag, tagValue, found := strings.Cut(part, "=")
				if !found {
					continue
				}
				tagValue = strings.Join(strings.Fields(tagValue), "")
				tags[strings.TrimSpace(tag)] = tagValue
			}
			return testSignature{tags: tags, field: field}, nil
		}
	}
	return testSignature{}, errors.New("no DKIM-Signature header")
}

func testHeaderData(fields []testField, signature testSignature) string {
	used := map[int]bool{}
	var builder strings.Builder
	for _, name := range strings.Split(signature.tags["h"], ":") {
		for index := len(fields) - 1; index >= 0; index-- {
			if used[index] || !strings.EqualFold(fields[index].name, name) {
				continue
			}
			used[index] = true
			builder.WriteString(testRelaxedHeader(fields[index].raw) + "\r\n")
			break
		}
	}
	emptied := regexp.MustCompile(`(^|;)(\s*b\s*=)[^;]*`).ReplaceAllString(signature.field.raw, "$1$2")
	builder.WriteString(testRelaxedHeader(emptied))
	return builder.String()
}

func testVerify(signed []byte, public crypto.PublicKey) (testSignature, error) {
	fields, body, err := testSplitMessage(signed)
	if err != nil {
		return testSignature{}, err
	}
	signature, err := testParseSignature(fields)
	if err != nil {
		return signature, err
	}
	if signature.tags["v"] != "1" || signature.tags["c"] != "relaxed/relaxed" {
		return signature, fmt.Errorf("bad v or c tag: %v", signature.tags)
	}
	bodyHash := sha256.Sum256(testRelaxedBody(body))
	if base64.StdEncoding.EncodeToString(bodyHash[:]) != signature.tags["bh"] {
		return signature, errors.New("body hash does not match")
	}
	digest := sha256.Sum256([]byte(testHeaderData(fields, signature)))
	signatureBytes, err := base64.StdEncoding.DecodeString(signature.tags["b"])
	if err != nil {
		return signature, err
	}
	return signature, testCheck(public, signature.tags["a"], digest[:], signatureBytes)
}

func testCheck(public crypto.PublicKey, algorithm string, digest []byte, signature []byte) error {
	switch key := public.(type) {
	case *rsa.PublicKey:
		if algorithm != "rsa-sha256" {
			return fmt.Errorf("algorithm %q for rsa", algorithm)
		}
		return rsa.VerifyPKCS1v15(key, crypto.SHA256, digest, signature)
	case ed25519.PublicKey:
		if algorithm != "ed25519-sha256" {
			return fmt.Errorf("algorithm %q for ed25519", algorithm)
		}
		if !ed25519.Verify(key, digest, signature) {
			return errors.New("ed25519 signature does not verify")
		}
		return nil
	}
	return errors.New("unknown key type")
}

func testPublicFromRecord(record string) (crypto.PublicKey, error) {
	tags := map[string]string{}
	for _, part := range strings.Split(record, ";") {
		tag, value, _ := strings.Cut(part, "=")
		tags[strings.TrimSpace(tag)] = strings.TrimSpace(value)
	}
	raw, err := base64.StdEncoding.DecodeString(tags["p"])
	if err != nil {
		return nil, err
	}
	if tags["k"] == "ed25519" {
		return ed25519.PublicKey(raw), nil
	}
	return x509.ParsePKIXPublicKey(raw)
}
