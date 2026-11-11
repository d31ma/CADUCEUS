package dkim

import "fmt"

type DnsRecord struct{}

func (DnsRecord) Text(key Key) (string, error) {
	chosen, public, err := methods{}.forKey(key)
	if err != nil {
		return "", err
	}
	encoded, err := chosen.publicKey(public)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("v=DKIM1; k=%s; p=%s", chosen.keyType(), encoded), nil
}
