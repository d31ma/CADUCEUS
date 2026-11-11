package mailauth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"slices"
	"strings"
	"time"
)

type dkimVerifier struct {
	lookup     dnsLookup
	receivedAt time.Time
	canonical  dkimCanonical
}

func (verifier dkimVerifier) verifyAll(ctx context.Context, message rawMessage) []DkimResult {
	var results []DkimResult
	for _, field := range verifier.signatureFields(message.fields) {
		results = append(results, verifier.check(ctx, message, field))
	}
	return results
}

func (verifier dkimVerifier) signatureFields(fields []messageField) []messageField {
	signatures := slices.DeleteFunc(slices.Clone(fields), verifier.notSignature)
	return signatures[:min(len(signatures), verifier.limit())]
}

func (verifier dkimVerifier) notSignature(field messageField) bool {
	return !field.named(verifier.fieldName())
}

func (dkimVerifier) fieldName() string {
	return "DKIM-Signature"
}

func (dkimVerifier) limit() int {
	return 5
}

func (verifier dkimVerifier) check(ctx context.Context, message rawMessage, field messageField) DkimResult {
	signature, result, err := verifier.run(ctx, message, field)
	return DkimResult{Result: failure{}.outcome(err, result), Domain: signature.domain, Selector: signature.selector}
}

func (verifier dkimVerifier) run(ctx context.Context, message rawMessage, field messageField) (dkimSignature, string, error) {
	signature, err := dkimSignature{}.parse(field)
	if err != nil {
		return signature, "", err
	}
	result, err := verifier.verify(ctx, message, signature)
	return signature, result, err
}

func (verifier dkimVerifier) verify(ctx context.Context, message rawMessage, signature dkimSignature) (string, error) {
	kind, err := dkimAlgorithms{}.kind(signature.algorithm)
	if err != nil {
		return "", err
	}
	err = verifier.current(signature)
	if err != nil {
		return "", err
	}
	body := verifier.canonical.body(message.body, signature.bodyMethod)
	err = verifier.bodyMatches(body, signature)
	if err != nil {
		return "", err
	}
	key, err := verifier.key(ctx, signature, kind)
	if err != nil {
		return "", err
	}
	err = verifier.signatureMatches(key, message, signature)
	if err != nil {
		return "", err
	}
	return verifier.lengthResult(body, signature), nil
}

func (verifier dkimVerifier) current(signature dkimSignature) error {
	if signature.hasExpiry && signature.expires < verifier.receivedAt.Unix() {
		return failure{result: Outcomes{}.Fail()}
	}
	return nil
}

func (verifier dkimVerifier) bodyMatches(body string, signature dkimSignature) error {
	limited, err := verifier.limited(body, signature)
	if err != nil {
		return err
	}
	digest := sha256.Sum256([]byte(limited))
	return verifier.same(digest[:], signature.bodyHash)
}

func (dkimVerifier) limited(body string, signature dkimSignature) (string, error) {
	switch {
	case !signature.hasLength:
		return body, nil
	case signature.length > int64(len(body)):
		return "", failure{result: Outcomes{}.Fail()}
	default:
		return body[:signature.length], nil
	}
}

func (dkimVerifier) same(expected []byte, actual []byte) error {
	if !bytes.Equal(expected, actual) {
		return failure{result: Outcomes{}.Fail()}
	}
	return nil
}

func (verifier dkimVerifier) key(ctx context.Context, signature dkimSignature, kind string) (signatureKey, error) {
	texts, err := verifier.lookup.text(ctx, signature.selector+"._domainkey."+signature.domain)
	if err != nil {
		return nil, err
	}
	record, err := dkimKey{}.parse(texts)
	if err != nil {
		return nil, err
	}
	signer, err := record.signer()
	if err != nil {
		return nil, err
	}
	return signer, verifier.matchingKind(signer, kind)
}

func (dkimVerifier) matchingKind(signer signatureKey, kind string) error {
	if signer.kind() != kind {
		return failure{result: Outcomes{}.Fail()}
	}
	return nil
}

func (verifier dkimVerifier) signatureMatches(key signatureKey, message rawMessage, signature dkimSignature) error {
	digest := sha256.Sum256(verifier.signedData(message, signature))
	return verifier.valid(key.verify(digest[:], signature.data))
}

func (dkimVerifier) valid(verified bool) error {
	if !verified {
		return failure{result: Outcomes{}.Fail()}
	}
	return nil
}

func (verifier dkimVerifier) signedData(message rawMessage, signature dkimSignature) []byte {
	picked := newDkimHeaderPicker(message.fields).pick(signature.headers)
	text := verifier.canonicalFields(picked, signature.headerMethod)
	own := verifier.canonical.header(verifier.canonical.withoutSignature(signature.field.raw), signature.headerMethod)
	return []byte(text + strings.TrimSuffix(own, verifier.canonical.newline()))
}

func (verifier dkimVerifier) canonicalFields(fields []messageField, method string) string {
	var text strings.Builder
	for _, field := range fields {
		text.WriteString(verifier.canonical.header(field.raw, method))
	}
	return text.String()
}

func (dkimVerifier) lengthResult(body string, signature dkimSignature) string {
	if signature.hasLength && signature.length < int64(len(body)) {
		return Outcomes{}.Policy()
	}
	return Outcomes{}.Pass()
}
