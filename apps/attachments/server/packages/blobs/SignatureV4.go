package blobs

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

type SignatureV4 struct {
	Settings S3Settings
}

func (signature SignatureV4) Presign(method string, target *url.URL, headers http.Header, now time.Time, lifetime time.Duration) string {
	stamp := now.UTC()
	names := signature.headerNames(headers)
	query := target.Query()
	query.Set("X-Amz-Algorithm", signature.algorithm())
	query.Set("X-Amz-Credential", signature.Settings.AccessKeyId+"/"+signature.scope(stamp))
	query.Set("X-Amz-Date", stamp.Format(signature.timeLayout()))
	query.Set("X-Amz-Expires", strconv.FormatInt(int64(lifetime/time.Second), 10))
	query.Set("X-Amz-SignedHeaders", strings.Join(names, ";"))
	signature.addSessionToken(query)
	encodedQuery := AwsEncoding{}.Query(query)
	canonical := CanonicalRequest{Method: method, Path: target.EscapedPath(), Query: encodedQuery, Headers: signature.canonicalHeaders(target.Host, headers, names), SignedHeaders: strings.Join(names, ";"), PayloadHash: signature.unsignedPayload()}
	return target.Scheme + "://" + target.Host + target.EscapedPath() + "?" + encodedQuery + "&X-Amz-Signature=" + signature.sign(canonical, stamp)
}

func (signature SignatureV4) Authorize(request *http.Request, payloadHash string, now time.Time) {
	stamp := now.UTC()
	request.Header.Set("X-Amz-Date", stamp.Format(signature.timeLayout()))
	request.Header.Set("X-Amz-Content-Sha256", payloadHash)
	signature.addSessionHeader(request.Header)
	names := signature.headerNames(request.Header)
	canonical := CanonicalRequest{Method: request.Method, Path: request.URL.EscapedPath(), Query: AwsEncoding{}.Query(request.URL.Query()), Headers: signature.canonicalHeaders(request.URL.Host, request.Header, names), SignedHeaders: strings.Join(names, ";"), PayloadHash: payloadHash}
	credential := signature.Settings.AccessKeyId + "/" + signature.scope(stamp)
	request.Header.Set("Authorization", signature.algorithm()+" Credential="+credential+", SignedHeaders="+strings.Join(names, ";")+", Signature="+signature.sign(canonical, stamp))
}

func (signature SignatureV4) sign(canonical CanonicalRequest, stamp time.Time) string {
	text := strings.Join([]string{signature.algorithm(), stamp.Format(signature.timeLayout()), signature.scope(stamp), canonical.Hash()}, "\n")
	return hex.EncodeToString(signature.hmac(signature.signingKey(stamp), text))
}

func (signature SignatureV4) signingKey(stamp time.Time) []byte {
	dateKey := signature.hmac([]byte("AWS4"+signature.Settings.SecretAccessKey), stamp.Format(signature.dateLayout()))
	regionKey := signature.hmac(dateKey, signature.Settings.Region)
	serviceKey := signature.hmac(regionKey, signature.service())
	return signature.hmac(serviceKey, signature.terminator())
}

func (SignatureV4) hmac(key []byte, text string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(text))
	return mac.Sum(nil)
}

func (signature SignatureV4) scope(stamp time.Time) string {
	return strings.Join([]string{stamp.Format(signature.dateLayout()), signature.Settings.Region, signature.service(), signature.terminator()}, "/")
}

func (signature SignatureV4) headerNames(headers http.Header) []string {
	names := []string{signature.hostHeader()}
	for name := range headers {
		names = append(names, strings.ToLower(name))
	}
	slices.Sort(names)
	return slices.Compact(names)
}

func (signature SignatureV4) canonicalHeaders(host string, headers http.Header, names []string) string {
	lines := ""
	for _, name := range names {
		lines += name + ":" + signature.headerValue(host, headers, name) + "\n"
	}
	return lines
}

func (signature SignatureV4) headerValue(host string, headers http.Header, name string) string {
	if name == signature.hostHeader() {
		return host
	}
	return strings.TrimSpace(headers.Get(name))
}

func (signature SignatureV4) addSessionToken(query url.Values) {
	if signature.Settings.SessionToken != "" {
		query.Set("X-Amz-Security-Token", signature.Settings.SessionToken)
	}
}

func (signature SignatureV4) addSessionHeader(headers http.Header) {
	if signature.Settings.SessionToken != "" {
		headers.Set("X-Amz-Security-Token", signature.Settings.SessionToken)
	}
}

func (SignatureV4) algorithm() string {
	return "AWS4-HMAC-SHA256"
}

func (SignatureV4) service() string {
	return "s3"
}

func (SignatureV4) terminator() string {
	return "aws4_request"
}

func (SignatureV4) hostHeader() string {
	return "host"
}

func (SignatureV4) unsignedPayload() string {
	return "UNSIGNED-PAYLOAD"
}

func (SignatureV4) timeLayout() string {
	return "20060102T150405Z"
}

func (SignatureV4) dateLayout() string {
	return "20060102"
}
