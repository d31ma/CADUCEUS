package mailauth

import (
	"cmp"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"slices"
	"strings"
)

type dkimKey struct {
	kind string
	data []byte
}

func (key dkimKey) parse(texts []string) (dkimKey, error) {
	text, err := key.first(texts)
	if err != nil {
		return key, err
	}
	tags, err := newTagList(text)
	if err != nil {
		return key, err
	}
	return key.fromTags(tags)
}

func (key dkimKey) fromTags(tags tagList) (dkimKey, error) {
	err := key.version(tags.compact("v"))
	if err != nil {
		return key, err
	}
	err = key.present(tags)
	if err != nil {
		return key, err
	}
	err = key.hashes(tags)
	if err != nil {
		return key, err
	}
	err = key.revoked(tags.compact("p"))
	if err != nil {
		return key, err
	}
	data, err := key.decode(tags.compact("p"))
	return dkimKey{kind: strings.ToLower(cmp.Or(tags.compact("k"), dkimAlgorithms{}.rsa())), data: data}, err
}

func (dkimKey) first(texts []string) (string, error) {
	if len(texts) == 0 {
		return "", failure{result: Outcomes{}.PermError()}
	}
	return texts[0], nil
}

func (dkimKey) version(version string) error {
	if version != "" && version != "DKIM1" {
		return failure{result: Outcomes{}.PermError()}
	}
	return nil
}

func (dkimKey) present(tags tagList) error {
	if tags.missing("p") {
		return failure{result: Outcomes{}.PermError()}
	}
	return nil
}

func (dkimKey) hashes(tags tagList) error {
	hashes := strings.Split(strings.ToLower(tags.compact("h")), ":")
	if tags.has("h") && !slices.Contains(hashes, "sha256") {
		return failure{result: Outcomes{}.Fail()}
	}
	return nil
}

func (dkimKey) revoked(data string) error {
	if data == "" {
		return failure{result: Outcomes{}.Fail()}
	}
	return nil
}

func (dkimKey) decode(text string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		return nil, failure{result: Outcomes{}.PermError()}
	}
	return data, nil
}

func (key dkimKey) signer() (signatureKey, error) {
	build, err := key.builder()
	if err != nil {
		return nil, err
	}
	return build(key.data)
}

func (key dkimKey) builder() (func([]byte) (signatureKey, error), error) {
	build, found := key.builders()[key.kind]
	if !found {
		return nil, failure{result: Outcomes{}.PermError()}
	}
	return build, nil
}

func (key dkimKey) builders() map[string]func([]byte) (signatureKey, error) {
	algorithms := dkimAlgorithms{}
	return map[string]func([]byte) (signatureKey, error){algorithms.rsa(): key.rsa, algorithms.ed25519(): key.ed25519}
}

func (key dkimKey) rsa(data []byte) (signatureKey, error) {
	parsed, err := key.publicKey(data)
	if err != nil {
		return nil, failure{result: Outcomes{}.PermError()}
	}
	return key.strongRsa(parsed)
}

func (dkimKey) publicKey(data []byte) (any, error) {
	parsed, err := x509.ParsePKIXPublicKey(data)
	if err == nil {
		return parsed, nil
	}
	return x509.ParsePKCS1PublicKey(data)
}

func (key dkimKey) strongRsa(parsed any) (signatureKey, error) {
	public, isRsa := parsed.(*rsa.PublicKey)
	if !isRsa || public.N.BitLen() < key.minimumBits() {
		return nil, failure{result: Outcomes{}.Fail()}
	}
	return rsaKey{public: public}, nil
}

func (dkimKey) minimumBits() int {
	return 1024
}

func (dkimKey) ed25519(data []byte) (signatureKey, error) {
	if len(data) != ed25519.PublicKeySize {
		return nil, failure{result: Outcomes{}.PermError()}
	}
	return ed25519Key{public: ed25519.PublicKey(data)}, nil
}
