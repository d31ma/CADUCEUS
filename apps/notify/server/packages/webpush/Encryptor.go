package webpush

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"encoding/binary"
	"io"
)

type Encryptor struct {
	random io.Reader
}

func NewEncryptor(random io.Reader) Encryptor {
	return Encryptor{random: random}
}

func (encryptor Encryptor) Encrypt(subscription Subscription, plaintext []byte) ([]byte, error) {
	err := encryptor.checkSize(plaintext)
	if err != nil {
		return nil, err
	}
	subscriber, err := SubscriberKeys{}.From(subscription)
	if err != nil {
		return nil, err
	}
	exchange, err := encryptor.exchange(subscriber)
	if err != nil {
		return nil, err
	}
	keys, err := ContentKeys{}.Derive(exchange)
	if err != nil {
		return nil, err
	}
	record, err := encryptor.seal(keys, plaintext)
	if err != nil {
		return nil, err
	}
	return append(encryptor.header(exchange), record...), nil
}

func (encryptor Encryptor) checkSize(plaintext []byte) error {
	if len(plaintext) > encryptor.maximumPlaintext() {
		return PayloadTooLarge{Size: len(plaintext), Limit: encryptor.maximumPlaintext()}
	}
	return nil
}

func (encryptor Encryptor) exchange(subscriber SubscriberKeys) (KeyExchange, error) {
	salt := make([]byte, encryptor.saltLength())
	_, err := io.ReadFull(encryptor.random, salt)
	if err != nil {
		return KeyExchange{}, err
	}
	serverKey, err := encryptor.serverKey()
	if err != nil {
		return KeyExchange{}, err
	}
	secret, err := serverKey.ECDH(subscriber.PublicKey)
	exchange := KeyExchange{SharedSecret: secret, AuthSecret: subscriber.AuthSecret, Salt: salt}
	exchange.UserAgentPublic = subscriber.PublicKey.Bytes()
	exchange.ServerPublic = serverKey.PublicKey().Bytes()
	return exchange, err
}

func (encryptor Encryptor) serverKey() (*ecdh.PrivateKey, error) {
	scalar := make([]byte, encryptor.scalarLength())
	_, err := io.ReadFull(encryptor.random, scalar)
	if err != nil {
		return nil, err
	}
	return ecdh.P256().NewPrivateKey(scalar)
}

func (encryptor Encryptor) seal(keys ContentKeys, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(keys.Key)
	if err != nil {
		return nil, err
	}
	authenticatedCipher, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	padded := append(append([]byte{}, plaintext...), encryptor.lastRecordDelimiter())
	return authenticatedCipher.Seal(nil, keys.Nonce, padded, nil), nil
}

func (encryptor Encryptor) header(exchange KeyExchange) []byte {
	header := append([]byte{}, exchange.Salt...)
	header = binary.BigEndian.AppendUint32(header, encryptor.recordSize())
	header = append(header, byte(len(exchange.ServerPublic)))
	return append(header, exchange.ServerPublic...)
}

func (Encryptor) maximumPlaintext() int {
	return 3993
}

func (Encryptor) recordSize() uint32 {
	return 4096
}

func (Encryptor) lastRecordDelimiter() byte {
	return 2
}

func (Encryptor) saltLength() int {
	return 16
}

func (Encryptor) scalarLength() int {
	return 32
}
