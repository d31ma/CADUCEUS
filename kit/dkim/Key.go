package dkim

import "crypto"

type Key struct {
	Domain   string
	Selector string
	Signer   crypto.Signer
}
