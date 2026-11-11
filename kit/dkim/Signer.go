package dkim

import (
	"crypto/sha256"
	"time"
)

type Signer struct{}

func (signer Signer) Sign(request SignRequest) ([]byte, error) {
	err := keyNames{}.checkKey(request.Key)
	if err != nil {
		return nil, err
	}
	chosen, _, err := methods{}.forKey(request.Key)
	if err != nil {
		return nil, err
	}
	parsed, err := messageReader{}.read(request.Message)
	if err != nil {
		return nil, err
	}
	err = fromDomains{}.check(parsed, request.Key)
	if err != nil {
		return nil, err
	}
	bodyHash := sha256.Sum256([]byte(relaxed{}.body(parsed.body)))
	header := signatureHeader{
		algorithm: chosen.algorithm(),
		domain:    request.Key.Domain,
		selector:  request.Key.Selector,
		timestamp: signer.timestamp(request.Time),
		headers:   headerNames{}.signed(parsed, headerNames{}.candidates(request.Headers)),
		bodyHash:  bodyHash[:],
	}
	digest := sha256.Sum256([]byte(headerNames{}.canonical(parsed, header.headers) + header.canonical()))
	signature, err := chosen.sign(request.Key.Signer, digest[:])
	if err != nil {
		return nil, UnsupportedKey{Reason: "the signer could not sign"}
	}
	return append([]byte(header.signed(signature)), request.Message...), nil
}

func (Signer) timestamp(moment time.Time) int64 {
	if moment.IsZero() {
		return time.Now().Unix()
	}
	return moment.Unix()
}
