package blobs

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func awsExampleSettings() S3Settings {
	return S3Settings{Region: "us-east-1", Bucket: "examplebucket", AccessKeyId: "AKIAIOSFODNN7EXAMPLE", SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"}
}

func awsExampleTime() time.Time {
	return time.Date(2013, 5, 24, 0, 0, 0, 0, time.UTC)
}

func TestPresignMatchesTheAwsDocumentedExample(t *testing.T) {
	target, _ := url.Parse("https://examplebucket.s3.amazonaws.com/test.txt")
	signed := SignatureV4{Settings: awsExampleSettings()}.Presign(http.MethodGet, target, http.Header{}, awsExampleTime(), 24*time.Hour)
	if !strings.HasSuffix(signed, "&X-Amz-Signature=aeeed9bbccd4d02ee5c0109b86d86835f995330da4c265957d157751f604d404") {
		t.Fatalf("got %s", signed)
	}
	if !strings.Contains(signed, "X-Amz-Credential=AKIAIOSFODNN7EXAMPLE%2F20130524%2Fus-east-1%2Fs3%2Faws4_request") {
		t.Fatalf("credential missing in %s", signed)
	}
}

func TestAuthorizeMatchesTheAwsDocumentedExample(t *testing.T) {
	request, _ := http.NewRequest(http.MethodGet, "https://examplebucket.s3.amazonaws.com/test.txt", nil)
	request.Header.Set("Range", "bytes=0-9")
	SignatureV4{Settings: awsExampleSettings()}.Authorize(request, S3Store{}.emptyPayloadHash(), awsExampleTime())
	want := "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request, SignedHeaders=host;range;x-amz-content-sha256;x-amz-date, Signature=f0e8bdb87c964420e857bd35b5d6ed310bd44f0170aba48dd91039c6036bdb41"
	if got := request.Header.Get("Authorization"); got != want {
		t.Fatalf("got %s", got)
	}
}

func TestSessionTokensAreSigned(t *testing.T) {
	settings := awsExampleSettings()
	settings.SessionToken = "token/with+chars"
	target, _ := url.Parse("https://examplebucket.s3.amazonaws.com/test.txt")
	signed := SignatureV4{Settings: settings}.Presign(http.MethodGet, target, http.Header{}, awsExampleTime(), time.Minute)
	request, _ := http.NewRequest(http.MethodGet, "https://examplebucket.s3.amazonaws.com/test.txt", nil)
	SignatureV4{Settings: settings}.Authorize(request, S3Store{}.emptyPayloadHash(), awsExampleTime())
	if !strings.Contains(signed, "X-Amz-Security-Token=token%2Fwith%2Bchars") || request.Header.Get("X-Amz-Security-Token") != "token/with+chars" {
		t.Fatalf("got %s and %v", signed, request.Header)
	}
	if !strings.Contains(request.Header.Get("Authorization"), "x-amz-security-token") {
		t.Fatalf("token header not signed: %s", request.Header.Get("Authorization"))
	}
}

func TestAwsEncodingUsesPercentTwentyAndSortsPairs(t *testing.T) {
	query := url.Values{"b": {"two words"}, "a": {"x~y", "a/b"}}
	if got := (AwsEncoding{}).Query(query); got != "a=a%2Fb&a=x~y&b=two%20words" {
		t.Fatalf("got %s", got)
	}
}

func TestBaseUrlDefaultsToAwsInTheRegion(t *testing.T) {
	if got := (S3Settings{Region: "eu-west-1"}).BaseUrl(); got != "https://s3.eu-west-1.amazonaws.com" {
		t.Fatalf("got %s", got)
	}
	if got := (S3Settings{Endpoint: "http://floci:4566"}).BaseUrl(); got != "http://floci:4566" {
		t.Fatalf("got %s", got)
	}
}
