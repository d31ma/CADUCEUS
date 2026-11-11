package webpush

import (
	"crypto/hkdf"
	"crypto/sha256"
	"errors"
)

type ContentKeys struct {
	Key   []byte
	Nonce []byte
}

func (keys ContentKeys) Derive(exchange KeyExchange) (ContentKeys, error) {
	inputKey, inputError := hkdf.Key(sha256.New, exchange.SharedSecret, exchange.AuthSecret, keys.keyInfo(exchange), keys.inputKeyLength())
	key, keyError := hkdf.Key(sha256.New, inputKey, exchange.Salt, "Content-Encoding: aes128gcm\x00", keys.keyLength())
	nonce, nonceError := hkdf.Key(sha256.New, inputKey, exchange.Salt, "Content-Encoding: nonce\x00", keys.nonceLength())
	return ContentKeys{Key: key, Nonce: nonce}, errors.Join(inputError, keyError, nonceError)
}

func (ContentKeys) keyInfo(exchange KeyExchange) string {
	return "WebPush: info\x00" + string(exchange.UserAgentPublic) + string(exchange.ServerPublic)
}

func (ContentKeys) inputKeyLength() int {
	return 32
}

func (ContentKeys) keyLength() int {
	return 16
}

func (ContentKeys) nonceLength() int {
	return 12
}
