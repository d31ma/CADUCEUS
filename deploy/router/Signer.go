package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"sort"
	"strings"
	"time"
)

type Signer struct {
	Credentials Credentials
	Service     string
}

func (signer Signer) Sign(request *http.Request, payload []byte, now time.Time) {
	stamp := now.UTC().Format("20060102T150405Z")
	day := stamp[:8]
	payloadHash := signer.hash(payload)
	request.Header.Set("X-Amz-Date", stamp)
	signer.addContentHash(request, payloadHash)
	signer.addSessionToken(request)
	names := signer.signedNames(request)
	canonical := strings.Join([]string{
		request.Method,
		signer.canonicalPath(request),
		signer.canonicalQuery(request),
		signer.canonicalHeaders(request, names),
		strings.Join(names, ";"),
		payloadHash,
	}, "\n")
	scope := strings.Join([]string{day, signer.Credentials.Region, signer.Service, "aws4_request"}, "/")
	toSign := strings.Join([]string{signer.algorithm(), stamp, scope, signer.hash([]byte(canonical))}, "\n")
	signature := hex.EncodeToString(signer.mac(signer.key(day), toSign))
	request.Header.Set("Authorization", signer.algorithm()+" Credential="+signer.Credentials.AccessKey+"/"+scope+
		", SignedHeaders="+strings.Join(names, ";")+", Signature="+signature)
}

func (Signer) algorithm() string {
	return "AWS4-HMAC-SHA256"
}

func (signer Signer) addContentHash(request *http.Request, payloadHash string) {
	if signer.Service == "s3" {
		request.Header.Set("X-Amz-Content-Sha256", payloadHash)
	}
}

func (signer Signer) addSessionToken(request *http.Request) {
	if signer.Credentials.SessionToken != "" {
		request.Header.Set("X-Amz-Security-Token", signer.Credentials.SessionToken)
	}
}

func (signer Signer) signedNames(request *http.Request) []string {
	names := []string{"host"}
	for name := range request.Header {
		names = signer.withAmazonName(names, strings.ToLower(name))
	}
	sort.Strings(names)
	return names
}

func (Signer) withAmazonName(names []string, name string) []string {
	if strings.HasPrefix(name, "x-amz-") {
		return append(names, name)
	}
	return names
}

func (signer Signer) canonicalHeaders(request *http.Request, names []string) string {
	var lines strings.Builder
	for _, name := range names {
		lines.WriteString(name + ":" + signer.headerValue(request, name) + "\n")
	}
	return lines.String()
}

func (Signer) headerValue(request *http.Request, name string) string {
	if name == "host" {
		return request.URL.Host
	}
	return strings.TrimSpace(strings.Join(request.Header.Values(name), ","))
}

func (Signer) canonicalPath(request *http.Request) string {
	path := request.URL.EscapedPath()
	if path == "" {
		return "/"
	}
	return path
}

func (Signer) canonicalQuery(request *http.Request) string {
	return strings.ReplaceAll(request.URL.Query().Encode(), "+", "%20")
}

func (signer Signer) key(day string) []byte {
	dated := signer.mac([]byte("AWS4"+signer.Credentials.SecretKey), day)
	regional := signer.mac(dated, signer.Credentials.Region)
	service := signer.mac(regional, signer.Service)
	return signer.mac(service, "aws4_request")
}

func (Signer) mac(key []byte, text string) []byte {
	digest := hmac.New(sha256.New, key)
	digest.Write([]byte(text))
	return digest.Sum(nil)
}

func (Signer) hash(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}
