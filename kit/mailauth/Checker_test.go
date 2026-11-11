package mailauth

import (
	"context"
	"net"
	"testing"
)

func TestCheckMarksSpfSoftFailAsSuspicious(t *testing.T) {
	fake := newFakeResolver().text("outside.test", "v=spf1 ~all")
	results := NewChecker(fake).Check(context.Background(), dmarcInput("ana@outside.test", "outside.test"))
	if results.Reject || !results.Suspicious {
		t.Fatalf("got %+v", results)
	}
}

func TestCheckMarksUnverifiedSignaturesAsSuspicious(t *testing.T) {
	signer := newEdSigner(t)
	fake := newFakeResolver().text("outside.test", "v=spf1 +all")
	input := dmarcInput("ana@outside.test", "outside.test")
	input.Raw = []byte(signer.sign(t, string(input.Raw)))
	results := NewChecker(fake).Check(context.Background(), input)
	if results.Spf != "pass" || results.DkimSummary() != "permerror" || !results.Suspicious {
		t.Fatalf("got %+v", results)
	}
}

func TestCheckIsQuietForAnUnsignedMessageWithoutRecords(t *testing.T) {
	results := NewChecker(newFakeResolver()).Check(context.Background(), dmarcInput("ana@outside.test", "outside.test"))
	if results.Spf != "none" || results.Dmarc != "none" || results.Suspicious || results.Reject {
		t.Fatalf("got %+v", results)
	}
}

func TestFailureReadsAsItsResult(t *testing.T) {
	if got := (failure{result: "temperror"}).Error(); got != "temperror" {
		t.Fatalf("got %q", got)
	}
}

func FuzzCheck(f *testing.F) {
	f.Add([]byte("From: Ana <ana@outside.test>\r\nDKIM-Signature: v=1; a=rsa-sha256; d=outside.test; s=s1; h=from; l=3; bh=AAAA; b=AAAA\r\n\r\nHi\r\n"))
	f.Add([]byte("From: =?utf-8?q?x?= <a@b.c>\nSubject: x\n\tfolded\n\nbody"))
	f.Add([]byte("\r\n\r\n"))
	f.Add([]byte("DKIM-Signature: ;;;==\r\nFrom:\r\n\r\n"))
	f.Fuzz(func(t *testing.T, raw []byte) {
		fake := newFakeResolver().
			text("outside.test", "v=spf1 include:_spf.outside.test mx a exists:%{i}.x.test ~all").
			text("_spf.outside.test", "v=spf1 ip4:192.0.2.0/24 redirect=outside.test").
			text("s1._domainkey.outside.test", "v=DKIM1; p=AAAA").
			text("_dmarc.outside.test", "v=DMARC1; p=reject; sp=quarantine; np=none; psd=n").
			exchange("outside.test", "mx.outside.test").
			address("mx.outside.test", "192.0.2.1")
		input := Input{RemoteIp: net.ParseIP("192.0.2.10"), Helo: "mx.outside.test", MailFrom: "a@outside.test", Raw: raw}
		results := NewChecker(fake).Check(context.Background(), input)
		_ = results.HeaderField("mx.example.com")
		_ = results.DkimSummary()
	})
}
