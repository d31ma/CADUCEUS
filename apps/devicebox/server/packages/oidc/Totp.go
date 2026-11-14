package oidc

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"slices"
	"time"
)

type Totp struct{}

func (totp Totp) NewSecret() (string, error) {
	secret := make([]byte, totp.values().SecretBytes)
	_, err := rand.Read(secret)
	return totp.encoding().EncodeToString(secret), err
}

func (totp Totp) Code(secret string, at time.Time) (string, error) {
	key, err := totp.encoding().DecodeString(secret)
	if err != nil {
		return "", err
	}
	return totp.codeAtStep(key, at.Unix()/totp.values().PeriodSeconds), nil
}

func (totp Totp) Matches(secret string, code string, at time.Time) bool {
	window, err := totp.window(secret, at)
	return totp.contains(window, code, err)
}

func (totp Totp) Uri(account string, secret string) string {
	label := url.PathEscape(totp.issuer() + ":" + account)
	query := url.Values{"secret": {secret}, "issuer": {totp.issuer()}, "algorithm": {"SHA1"}, "digits": {fmt.Sprint(totp.values().Digits)}, "period": {fmt.Sprint(totp.values().PeriodSeconds)}}
	return "otpauth://totp/" + label + "?" + query.Encode()
}

func (totp Totp) window(secret string, at time.Time) ([]string, error) {
	key, err := totp.encoding().DecodeString(secret)
	if err != nil {
		return nil, err
	}
	step := at.Unix() / totp.values().PeriodSeconds
	return []string{totp.codeAtStep(key, step-1), totp.codeAtStep(key, step), totp.codeAtStep(key, step+1)}, nil
}

func (Totp) contains(window []string, code string, err error) bool {
	return err == nil && slices.ContainsFunc(window, func(candidate string) bool {
		return subtle.ConstantTimeCompare([]byte(candidate), []byte(code)) == 1
	})
}

func (totp Totp) codeAtStep(key []byte, step int64) string {
	counter := make([]byte, totp.values().CounterBytes)
	binary.BigEndian.PutUint64(counter, uint64(step))
	mac := hmac.New(sha1.New, key)
	mac.Write(counter)
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & totp.values().OffsetMask
	value := binary.BigEndian.Uint32(sum[offset:offset+totp.values().ValueBytes]) & totp.values().ValueMask
	return fmt.Sprintf("%0*d", totp.values().Digits, value%totp.values().Modulus)
}

func (Totp) encoding() *base32.Encoding {
	return base32.StdEncoding.WithPadding(base32.NoPadding)
}

func (Totp) issuer() string {
	return "Mesail"
}

func (Totp) values() TotpValues {
	return TotpValues{
		SecretBytes:   20,
		PeriodSeconds: 30,
		Digits:        6,
		Modulus:       1000000,
		CounterBytes:  8,
		ValueBytes:    4,
		OffsetMask:    0x0f,
		ValueMask:     0x7fffffff,
	}
}
