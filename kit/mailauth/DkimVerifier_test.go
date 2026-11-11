package mailauth

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"math/big"
	"net"
	"strings"
	"testing"
	"time"
)

func dkimMessage() string {
	return "From: Ana <ana@outside.test>\r\nTo: bo@inside.test\r\nSubject: Hello   there\r\n" +
		"Message-ID: <1@outside.test>\r\n\r\nHello world.  \r\nSecond \t line.\r\n\r\n\r\n"
}

func dkimReceivedAt() time.Time {
	return time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
}

func dkimCheck(fake *FakeResolver, raw string) []DkimResult {
	input := Input{RemoteIp: net.ParseIP("192.0.2.10"), MailFrom: "ana@outside.test", Raw: []byte(raw), ReceivedAt: dkimReceivedAt()}
	return NewChecker(fake).Check(context.Background(), input).Dkim
}

func expectDkim(t *testing.T, results []DkimResult, want string) {
	t.Helper()
	if len(results) != 1 || results[0].Result != want {
		t.Fatalf("dkim: got %+v, want %q", results, want)
	}
}

func TestDkimPassesRelaxedRsa(t *testing.T) {
	signer := newRsaSigner(t)
	results := dkimCheck(signer.publish(t, newFakeResolver()), signer.sign(t, dkimMessage()))
	expectDkim(t, results, "pass")
	if results[0].Domain != "outside.test" || results[0].Selector != "s1" {
		t.Fatalf("got %+v", results[0])
	}
}

func TestDkimPassesSimpleRsa(t *testing.T) {
	signer := newRsaSigner(t)
	signer.canonical = "simple/simple"
	expectDkim(t, dkimCheck(signer.publish(t, newFakeResolver()), signer.sign(t, dkimMessage())), "pass")
}

func TestDkimPassesEd25519(t *testing.T) {
	signer := newEdSigner(t)
	expectDkim(t, dkimCheck(signer.publish(t, newFakeResolver()), signer.sign(t, dkimMessage())), "pass")
}

func TestDkimPassesWithLineFeedOnlyMessage(t *testing.T) {
	signer := newEdSigner(t)
	signed := strings.ReplaceAll(signer.sign(t, dkimMessage()), "\r\n", "\n")
	expectDkim(t, dkimCheck(signer.publish(t, newFakeResolver()), signed), "pass")
}

func TestDkimRelaxedToleratesWhitespaceChanges(t *testing.T) {
	signer := newEdSigner(t)
	signed := signer.sign(t, dkimMessage())
	changed := strings.Replace(strings.Replace(signed, "Subject: Hello   there", "subject:Hello there ", 1), "Hello world.  ", "Hello  world.", 1)
	expectDkim(t, dkimCheck(signer.publish(t, newFakeResolver()), changed), "pass")
}

func TestDkimSimpleRefusesWhitespaceChanges(t *testing.T) {
	signer := newEdSigner(t)
	signer.canonical = "simple/simple"
	changed := strings.Replace(signer.sign(t, dkimMessage()), "Subject: Hello   there", "Subject: Hello there", 1)
	expectDkim(t, dkimCheck(signer.publish(t, newFakeResolver()), changed), "fail")
}

func TestDkimFailsWhenTheBodyChanged(t *testing.T) {
	signer := newRsaSigner(t)
	changed := strings.Replace(signer.sign(t, dkimMessage()), "Second", "Third", 1)
	expectDkim(t, dkimCheck(signer.publish(t, newFakeResolver()), changed), "fail")
}

func TestDkimFailsWhenASignedHeaderChanged(t *testing.T) {
	signer := newRsaSigner(t)
	changed := strings.Replace(signer.sign(t, dkimMessage()), "Subject: Hello", "Subject: Bye", 1)
	expectDkim(t, dkimCheck(signer.publish(t, newFakeResolver()), changed), "fail")
}

func TestDkimOverSignedHeaderRefusesAnAddedField(t *testing.T) {
	signer := newEdSigner(t)
	signer.headers = "from:from:to:subject:subject:reply-to"
	signed := signer.sign(t, dkimMessage())
	expectDkim(t, dkimCheck(signer.publish(t, newFakeResolver()), signed), "pass")
	added := strings.Replace(signed, "\r\n\r\n", "\r\nSubject: added\r\n\r\n", 1)
	expectDkim(t, dkimCheck(signer.publish(t, newFakeResolver()), added), "fail")
}

func TestDkimFailsWhenExpired(t *testing.T) {
	signer := newEdSigner(t)
	signer.extra = fmt.Sprintf("t=%d; x=%d; ", dkimReceivedAt().Unix()-100, dkimReceivedAt().Unix()-1)
	expectDkim(t, dkimCheck(signer.publish(t, newFakeResolver()), signer.sign(t, dkimMessage())), "fail")
}

func TestDkimPassesBeforeExpiry(t *testing.T) {
	signer := newEdSigner(t)
	signer.extra = fmt.Sprintf("t=%d; x=%d; ", dkimReceivedAt().Unix()-100, dkimReceivedAt().Unix()+100)
	expectDkim(t, dkimCheck(signer.publish(t, newFakeResolver()), signer.sign(t, dkimMessage())), "pass")
}

func TestDkimFailsForARevokedKey(t *testing.T) {
	signer := newEdSigner(t)
	fake := newFakeResolver().text("e1._domainkey.outside.test", "v=DKIM1; k=ed25519; p=")
	expectDkim(t, dkimCheck(fake, signer.sign(t, dkimMessage())), "fail")
}

func TestDkimFailsForASmallRsaKey(t *testing.T) {
	signer := newRsaSigner(t)
	small := &rsa.PublicKey{N: new(big.Int).Lsh(big.NewInt(1), 511), E: 65537}
	small.N.Add(small.N, big.NewInt(1))
	der, err := x509.MarshalPKIXPublicKey(small)
	if err != nil {
		t.Fatal(err)
	}
	fake := newFakeResolver().text("s1._domainkey.outside.test", "v=DKIM1; p="+base64.StdEncoding.EncodeToString(der))
	expectDkim(t, dkimCheck(fake, signer.sign(t, dkimMessage())), "fail")
}

func TestDkimAcceptsAPkcs1RsaKey(t *testing.T) {
	signer := newRsaSigner(t)
	der := x509.MarshalPKCS1PublicKey(&signer.rsaKey.PublicKey)
	fake := newFakeResolver().text("s1._domainkey.outside.test", "p="+base64.StdEncoding.EncodeToString(der))
	expectDkim(t, dkimCheck(fake, signer.sign(t, dkimMessage())), "pass")
}

func TestDkimRefusesRsaSha1(t *testing.T) {
	signer := newRsaSigner(t)
	signer.algorithm = "rsa-sha1"
	expectDkim(t, dkimCheck(signer.publish(t, newFakeResolver()), signer.sign(t, dkimMessage())), "fail")
}

func TestDkimGivesPolicyForAShortLength(t *testing.T) {
	signer := newRsaSigner(t)
	signer.length = 10
	expectDkim(t, dkimCheck(signer.publish(t, newFakeResolver()), signer.sign(t, dkimMessage())), "policy")
}

func TestDkimPassesWithTheFullLength(t *testing.T) {
	signer := newEdSigner(t)
	signer.length = len(dkimCanonical{}.relaxedBody(newRawMessage([]byte(dkimMessage())).body))
	expectDkim(t, dkimCheck(signer.publish(t, newFakeResolver()), signer.sign(t, dkimMessage())), "pass")
}

func TestDkimFailsWhenTheLengthIsLongerThanTheBody(t *testing.T) {
	signer := newEdSigner(t)
	signer.length = 10
	signed := signer.sign(t, dkimMessage())
	cut := signed[:strings.Index(signed, "Hello world.")] + "Hi\r\n"
	expectDkim(t, dkimCheck(signer.publish(t, newFakeResolver()), cut), "fail")
}

func TestDkimGivesTempErrorWhenTheKeyLookupFails(t *testing.T) {
	signer := newEdSigner(t)
	fake := newFakeResolver().fail("e1._domainkey.outside.test")
	expectDkim(t, dkimCheck(fake, signer.sign(t, dkimMessage())), "temperror")
}

func TestDkimGivesPermErrorWithoutAKey(t *testing.T) {
	signer := newEdSigner(t)
	expectDkim(t, dkimCheck(newFakeResolver(), signer.sign(t, dkimMessage())), "permerror")
}

func TestDkimFailsWhenTheKeyTypeDoesNotMatch(t *testing.T) {
	signer := newEdSigner(t)
	other := newRsaSigner(t)
	fake := newFakeResolver().text("e1._domainkey.outside.test", other.keyRecord(t))
	expectDkim(t, dkimCheck(fake, signer.sign(t, dkimMessage())), "fail")
}

func TestDkimFailsWithAWrongKey(t *testing.T) {
	signer := newEdSigner(t)
	other := newEdSigner(t)
	fake := newFakeResolver().text("e1._domainkey.outside.test", other.keyRecord(t))
	expectDkim(t, dkimCheck(fake, signer.sign(t, dkimMessage())), "fail")
}

func TestDkimKeyRecordProblems(t *testing.T) {
	signer := newEdSigner(t)
	key := strings.TrimPrefix(signer.keyRecord(t), "v=DKIM1; k=ed25519; ")
	cases := map[string]string{
		"v=DKIM2; k=ed25519; " + key:       "permerror",
		"k=ed25519; h=sha1; " + key:        "fail",
		"k=ed25519; h=sha1:sha256; " + key: "pass",
		"k=ed25519; p=@@@":                 "permerror",
		"k=ed25519; p=AAAA":                "permerror",
		"k=dsa; " + key:                    "permerror",
		"k=rsa; p=AAAA":                    "permerror",
		"k=ed25519 " + key:                 "permerror",
	}
	for record, want := range cases {
		fake := newFakeResolver().text("e1._domainkey.outside.test", record)
		results := dkimCheck(fake, signer.sign(t, dkimMessage()))
		if results[0].Result != want {
			t.Errorf("%q: got %q, want %q", record, results[0].Result, want)
		}
	}
}

func TestDkimSignatureProblemsArePermErrors(t *testing.T) {
	fields := []string{
		"DKIM-Signature: v=1; a=rsa-sha256; d=outside.test; s=s1; h=from; bh=AAAA\r\n",
		"DKIM-Signature: v=2; a=rsa-sha256; d=outside.test; s=s1; h=from; bh=AAAA; b=AAAA\r\n",
		"DKIM-Signature: v=1; a=rsa-md5; d=outside.test; s=s1; h=from; bh=AAAA; b=AAAA\r\n",
		"DKIM-Signature: v=1; a=rsa-sha256; c=odd; d=outside.test; s=s1; h=from; bh=AAAA; b=AAAA\r\n",
		"DKIM-Signature: v=1; a=rsa-sha256; d=outside.test; s=s1; h=to; bh=AAAA; b=AAAA\r\n",
		"DKIM-Signature: v=1; a=rsa-sha256; d=outside.test; s=s1; h=from; bh=AAAA; b=@@\r\n",
		"DKIM-Signature: v=1; a=rsa-sha256; d=outside.test; s=s1; h=from; bh=@@; b=AAAA\r\n",
		"DKIM-Signature: v=1; a=rsa-sha256; d=outside.test; s=s1; h=from; l=-1; bh=AAAA; b=AAAA\r\n",
		"DKIM-Signature: v=1; a=rsa-sha256; d=outside.test; s=s1; h=from; x=z; bh=AAAA; b=AAAA\r\n",
		"DKIM-Signature: v=1; a=rsa-sha256; d=outside.test; s=s1; h=from; t=z; bh=AAAA; b=AAAA\r\n",
		"DKIM-Signature: v=1; a=rsa-sha256; d=outside.test; s=s1; h=from; t=10; x=5; bh=AAAA; b=AAAA\r\n",
		"DKIM-Signature: v=1; a=rsa-sha256; d=outside.test; s=s1; h=from; i=a@other.test; bh=AAAA; b=AAAA\r\n",
		"DKIM-Signature: v=1; v=1; a=rsa-sha256; d=outside.test; s=s1; h=from; bh=AAAA; b=AAAA\r\n",
		"DKIM-Signature: v=1; broken; d=outside.test; s=s1\r\n",
	}
	for _, field := range fields {
		results := dkimCheck(newFakeResolver(), field+dkimMessage())
		if len(results) != 1 || results[0].Result != "permerror" {
			t.Errorf("%q: got %+v", field, results)
		}
	}
}

func TestDkimAcceptsAnIdentityInASubdomain(t *testing.T) {
	signer := newEdSigner(t)
	signer.extra = "i=ana@mail.outside.test; "
	expectDkim(t, dkimCheck(signer.publish(t, newFakeResolver()), signer.sign(t, dkimMessage())), "pass")
}

func TestDkimChecksAtMostFiveSignatures(t *testing.T) {
	signer := newEdSigner(t)
	signed := signer.sign(t, dkimMessage())
	field := signed[:strings.Index(signed, "From:")]
	results := dkimCheck(signer.publish(t, newFakeResolver()), strings.Repeat(field, 7)+dkimMessage())
	if len(results) != 5 || results[4].Result != "pass" {
		t.Fatalf("got %+v", results)
	}
}

func TestDkimHasNoResultsWithoutSignatures(t *testing.T) {
	if results := dkimCheck(newFakeResolver(), dkimMessage()); len(results) != 0 {
		t.Fatalf("got %+v", results)
	}
}

func TestDkimCanonicalHeadersMatchTheRfcExample(t *testing.T) {
	canonical := dkimCanonical{}
	relaxed := canonical.relaxedHeader("A: X\r\n") + canonical.relaxedHeader("B : Y\t\r\n\tZ  \r\n")
	simple := canonical.simpleHeader("A: X\r\n") + canonical.simpleHeader("B : Y\t\r\n\tZ  \r\n")
	if relaxed != "a:X\r\nb:Y Z\r\n" || simple != "A: X\r\nB : Y\t\r\n\tZ  \r\n" {
		t.Fatalf("got %q and %q", relaxed, simple)
	}
}

func TestDkimCanonicalBodiesMatchTheRfcExample(t *testing.T) {
	canonical := dkimCanonical{}
	body := " C \r\nD \t E\r\n\r\n\r\n"
	if got := canonical.relaxedBody(body); got != " C\r\nD E\r\n" {
		t.Fatalf("relaxed got %q", got)
	}
	if got := canonical.simpleBody(body); got != " C \r\nD \t E\r\n" {
		t.Fatalf("simple got %q", got)
	}
}

func TestDkimCanonicalEmptyBodies(t *testing.T) {
	canonical := dkimCanonical{}
	if canonical.relaxedBody("\r\n\r\n") != "" || canonical.simpleBody("") != "\r\n" || canonical.relaxedBody("a  ") != "a\r\n" {
		t.Fatal("empty body canonicalization is wrong")
	}
}

func TestDkimRemovesOnlyTheSignatureValue(t *testing.T) {
	got := dkimCanonical{}.withoutSignature("DKIM-Signature: b=abc; bh=xyz;\r\n\tb =\r\n de\r\n")
	if got != "DKIM-Signature: b=; bh=xyz;\r\n\tb =" {
		t.Fatalf("got %q", got)
	}
}
