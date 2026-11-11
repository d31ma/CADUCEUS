package dkim

import (
	"errors"
	"io/fs"
)

type KeyStore struct {
	Source SecretSource
}

func (store KeyStore) Key(domain string, selector string) (Key, error) {
	err := keyNames{}.checkKey(Key{Domain: domain, Selector: selector})
	if err != nil {
		return Key{}, err
	}
	secret, err := store.Source.Secret(keyNames{}.secretName(domain, selector))
	if err != nil {
		return Key{}, store.secretError(err, MissingKey{Domain: domain, Selector: selector})
	}
	signer, err := pemReader{}.signer(secret)
	if err != nil {
		return Key{}, err
	}
	key := Key{Domain: domain, Selector: selector, Signer: signer}
	_, _, err = methods{}.forKey(key)
	if err != nil {
		return Key{}, err
	}
	return key, nil
}

func (KeyStore) secretError(err error, missing MissingKey) error {
	if errors.Is(err, fs.ErrNotExist) {
		return missing
	}
	return err
}
