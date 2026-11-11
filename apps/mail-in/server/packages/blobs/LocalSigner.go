package blobs

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type LocalSigner struct {
	secret []byte
}

func NewLocalSigner(secret []byte) LocalSigner {
	return LocalSigner{secret: secret}
}

func (signer LocalSigner) Query(grant Grant) url.Values {
	query := url.Values{}
	query.Set("expires", strconv.FormatInt(grant.ExpiresAt, 10))
	query.Set("type", grant.MediaType)
	query.Set("size", strconv.FormatInt(grant.Size, 10))
	query.Set("file", grant.FileName)
	query.Set("signature", signer.signature(grant.Method, grant.Key, query))
	return query
}

func (signer LocalSigner) Grant(method string, key Key, query url.Values, now time.Time) (Grant, error) {
	err := signer.checkSignature(signer.signature(method, key, query), query.Get("signature"))
	if err != nil {
		return Grant{}, err
	}
	grant := signer.parsed(method, key, query)
	return grant, signer.checkExpiry(grant, now)
}

func (signer LocalSigner) signature(method string, key Key, query url.Values) string {
	canonical := strings.Join([]string{method, key.Path(), query.Get("expires"), query.Get("type"), query.Get("size"), query.Get("file")}, "\n")
	mac := hmac.New(sha256.New, signer.secret)
	mac.Write([]byte(canonical))
	return hex.EncodeToString(mac.Sum(nil))
}

func (LocalSigner) parsed(method string, key Key, query url.Values) Grant {
	expiresAt, _ := strconv.ParseInt(query.Get("expires"), 10, 64)
	size, _ := strconv.ParseInt(query.Get("size"), 10, 64)
	return Grant{Method: method, Key: key, ExpiresAt: expiresAt, MediaType: query.Get("type"), Size: size, FileName: query.Get("file")}
}

func (LocalSigner) checkSignature(expected string, given string) error {
	if !hmac.Equal([]byte(expected), []byte(given)) {
		return BadSignature{}
	}
	return nil
}

func (LocalSigner) checkExpiry(grant Grant, now time.Time) error {
	if now.Unix() > grant.ExpiresAt {
		return Expired{}
	}
	return nil
}
