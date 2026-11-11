package webpush

import (
	"crypto/ecdh"
	"errors"
)

type SubscriberKeys struct {
	PublicKey  *ecdh.PublicKey
	AuthSecret []byte
}

func (keys SubscriberKeys) From(subscription Subscription) (SubscriberKeys, error) {
	publicKey, err := keys.publicKey(subscription.P256dh)
	if err != nil {
		return SubscriberKeys{}, err
	}
	authSecret, err := keys.authSecret(subscription.Auth)
	if err != nil {
		return SubscriberKeys{}, err
	}
	return SubscriberKeys{PublicKey: publicKey, AuthSecret: authSecret}, nil
}

func (keys SubscriberKeys) publicKey(value string) (*ecdh.PublicKey, error) {
	raw, decodeError := Base64Text{}.Decode(value)
	key, keyError := ecdh.P256().NewPublicKey(raw)
	return key, keys.invalid(keys.publicField(), errors.Join(decodeError, keyError))
}

func (keys SubscriberKeys) authSecret(value string) ([]byte, error) {
	raw, err := Base64Text{}.Decode(value)
	if err != nil || len(raw) != keys.authSecretLength() {
		return nil, InvalidSubscription{Field: keys.authField()}
	}
	return raw, nil
}

func (SubscriberKeys) invalid(field string, err error) error {
	if err != nil {
		return InvalidSubscription{Field: field}
	}
	return nil
}

func (SubscriberKeys) authSecretLength() int {
	return 16
}

func (SubscriberKeys) publicField() string {
	return "p256dh"
}

func (SubscriberKeys) authField() string {
	return "auth"
}
