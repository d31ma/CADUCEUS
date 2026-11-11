package mailauth

import (
	"context"
	"net"
	"slices"
	"testing"
)

func dmarcInput(mailFrom string, from string) Input {
	raw := "From: Ana <ana@" + from + ">\r\nTo: bo@inside.test\r\nSubject: Hi\r\n\r\nHello.\r\n"
	return Input{RemoteIp: net.ParseIP("192.0.2.10"), Helo: "mail.outside.test", MailFrom: mailFrom, Raw: []byte(raw), ReceivedAt: dkimReceivedAt()}
}

func dmarcCheck(fake *FakeResolver, input Input) Results {
	return NewChecker(fake).Check(context.Background(), input)
}

func expectDmarc(t *testing.T, results Results, dmarc string, policy string) {
	t.Helper()
	if results.Dmarc != dmarc || results.DmarcPolicy != policy {
		t.Fatalf("got dmarc=%q policy=%q (%+v), want %q %q", results.Dmarc, results.DmarcPolicy, results, dmarc, policy)
	}
}

func TestDmarcPassesWithAlignedSpf(t *testing.T) {
	fake := newFakeResolver().text("outside.test", "v=spf1 ip4:192.0.2.10 -all").text("_dmarc.outside.test", "v=DMARC1; p=reject")
	results := dmarcCheck(fake, dmarcInput("ana@outside.test", "outside.test"))
	expectDmarc(t, results, "pass", "reject")
	if results.Reject || results.Suspicious || results.FromDomain != "outside.test" {
		t.Fatalf("got %+v", results)
	}
}

func TestDmarcPassesWithAlignedDkim(t *testing.T) {
	signer := newEdSigner(t)
	fake := signer.publish(t, newFakeResolver()).text("_dmarc.outside.test", "v=DMARC1; p=reject")
	input := dmarcInput("bounce@elsewhere.test", "outside.test")
	input.Raw = []byte(signer.sign(t, string(input.Raw)))
	results := dmarcCheck(fake, input)
	expectDmarc(t, results, "pass", "reject")
	if results.Reject || results.Suspicious {
		t.Fatalf("got %+v", results)
	}
}

func TestDmarcFailureWithRejectPolicyIsRejected(t *testing.T) {
	fake := newFakeResolver().text("outside.test", "v=spf1 -all").text("_dmarc.outside.test", "v=DMARC1; p=reject")
	results := dmarcCheck(fake, dmarcInput("ana@outside.test", "outside.test"))
	expectDmarc(t, results, "fail", "reject")
	if !results.Reject || results.Suspicious {
		t.Fatalf("got %+v", results)
	}
}

func TestDmarcFailureWithQuarantinePolicyIsSuspicious(t *testing.T) {
	fake := newFakeResolver().text("_dmarc.outside.test", "v=DMARC1; p=quarantine; rua=mailto:x@outside.test; pct=50")
	results := dmarcCheck(fake, dmarcInput("ana@elsewhere.test", "outside.test"))
	expectDmarc(t, results, "fail", "quarantine")
	if results.Reject || !results.Suspicious {
		t.Fatalf("got %+v", results)
	}
}

func TestDmarcSubdomainUsesTheSubdomainPolicy(t *testing.T) {
	fake := newFakeResolver().text("_dmarc.outside.test", "v=DMARC1; p=reject; sp=quarantine").address("news.outside.test", "192.0.2.1")
	expectDmarc(t, dmarcCheck(fake, dmarcInput("", "news.outside.test")), "fail", "quarantine")
}

func TestDmarcSubdomainWithoutSubdomainPolicyUsesThePolicy(t *testing.T) {
	fake := newFakeResolver().text("_dmarc.outside.test", "v=DMARC1; p=reject; np=none").address("news.outside.test", "192.0.2.1")
	expectDmarc(t, dmarcCheck(fake, dmarcInput("", "news.outside.test")), "fail", "reject")
}

func TestDmarcNonexistentSubdomainUsesTheNonexistentPolicy(t *testing.T) {
	fake := newFakeResolver().text("_dmarc.outside.test", "v=DMARC1; p=reject; sp=none; np=quarantine")
	expectDmarc(t, dmarcCheck(fake, dmarcInput("", "missing.outside.test")), "fail", "quarantine")
}

func TestDmarcTreeWalkFindsTheOrganizationPolicy(t *testing.T) {
	fake := newFakeResolver().text("_dmarc.outside.test", "v=DMARC1; p=reject; sp=quarantine").text("_dmarc.c.outside.test", "not dmarc")
	results := dmarcCheck(fake, dmarcInput("", "a.b.c.outside.test"))
	expectDmarc(t, results, "fail", "quarantine")
	for _, name := range []string{"_dmarc.a.b.c.outside.test", "_dmarc.b.c.outside.test", "_dmarc.c.outside.test", "_dmarc.outside.test"} {
		if !slices.Contains(fake.asked, name) {
			t.Fatalf("did not ask %q: %v", name, fake.asked)
		}
	}
}

func TestDmarcTreeWalkShortensLongNamesToSevenLabels(t *testing.T) {
	fake := newFakeResolver()
	dmarcCheck(fake, dmarcInput("", "1.2.3.4.5.6.7.8.9.test"))
	if !slices.Contains(fake.asked, "_dmarc.4.5.6.7.8.9.test") || slices.Contains(fake.asked, "_dmarc.3.4.5.6.7.8.9.test") {
		t.Fatalf("asked %v", fake.asked)
	}
}

func TestDmarcRelaxedAlignmentAcceptsASubdomain(t *testing.T) {
	fake := newFakeResolver().text("bounce.outside.test", "v=spf1 +all").text("_dmarc.outside.test", "v=DMARC1; p=reject")
	expectDmarc(t, dmarcCheck(fake, dmarcInput("x@bounce.outside.test", "outside.test")), "pass", "reject")
}

func TestDmarcStrictAlignmentRefusesASubdomain(t *testing.T) {
	fake := newFakeResolver().text("bounce.outside.test", "v=spf1 +all").text("_dmarc.outside.test", "v=DMARC1; p=reject; aspf=s")
	expectDmarc(t, dmarcCheck(fake, dmarcInput("x@bounce.outside.test", "outside.test")), "fail", "reject")
}

func TestDmarcRelaxedAlignmentRefusesAnotherOrganization(t *testing.T) {
	fake := newFakeResolver().text("other.test", "v=spf1 +all").text("_dmarc.outside.test", "v=DMARC1; p=reject").fail("_dmarc.other.test")
	expectDmarc(t, dmarcCheck(fake, dmarcInput("x@other.test", "outside.test")), "fail", "reject")
}

func TestDmarcPublicSuffixRecordSetsTheOrganization(t *testing.T) {
	fake := newFakeResolver().text("b.outside.test", "v=spf1 +all").text("_dmarc.test", "v=DMARC1; p=reject; psd=y")
	expectDmarc(t, dmarcCheck(fake, dmarcInput("x@b.outside.test", "a.outside.test")), "pass", "reject")
}

func TestDmarcPsdNoStopsTheWalk(t *testing.T) {
	fake := newFakeResolver().text("b.outside.test", "v=spf1 +all").
		text("_dmarc.a.outside.test", "v=DMARC1; p=quarantine; psd=n").
		text("_dmarc.outside.test", "v=DMARC1; p=reject")
	expectDmarc(t, dmarcCheck(fake, dmarcInput("x@b.outside.test", "a.outside.test")), "fail", "quarantine")
}

func TestDmarcIsNoneWithoutARecord(t *testing.T) {
	fake := newFakeResolver().text("_dmarc.outside.test", "v=DMARC1; p=reject", "v=DMARC1; p=none").text("_dmarc.test", "v=DMARC1; p=reject; p=none")
	results := dmarcCheck(fake, dmarcInput("ana@elsewhere.test", "outside.test"))
	expectDmarc(t, results, "none", "none")
	if results.Reject || results.Suspicious {
		t.Fatalf("got %+v", results)
	}
}

func TestDmarcGivesTempErrorWhenDnsFails(t *testing.T) {
	fake := newFakeResolver().fail("_dmarc.outside.test")
	expectDmarc(t, dmarcCheck(fake, dmarcInput("ana@outside.test", "outside.test")), "temperror", "none")
}

func TestDmarcIsNoneWithoutAFromField(t *testing.T) {
	input := dmarcInput("ana@outside.test", "outside.test")
	input.Raw = []byte("To: bo@inside.test\r\n\r\nHello.\r\n")
	expectDmarc(t, dmarcCheck(newFakeResolver(), input), "none", "none")
}

func TestDmarcGivesPermErrorForUnclearAuthors(t *testing.T) {
	raws := []string{
		"From: a@outside.test\r\nFrom: b@outside.test\r\n\r\nHi\r\n",
		"From: a@outside.test, b@other.test\r\n\r\nHi\r\n",
		"From: not an address\r\n\r\nHi\r\n",
	}
	for _, raw := range raws {
		input := dmarcInput("ana@outside.test", "outside.test")
		input.Raw = []byte(raw)
		results := dmarcCheck(newFakeResolver(), input)
		if results.Dmarc != "permerror" || results.FromDomain != "" {
			t.Errorf("%q: got %+v", raw, results)
		}
	}
}

func TestDmarcReadsAnEncodedDisplayName(t *testing.T) {
	input := dmarcInput("ana@outside.test", "outside.test")
	input.Raw = []byte("From: =?koi8-r?B?4eTB?= <Ana@Outside.Test>\r\n\r\nHi\r\n")
	fake := newFakeResolver().text("outside.test", "v=spf1 +all").text("_dmarc.outside.test", "v=DMARC1; p=none")
	expectDmarc(t, dmarcCheck(fake, input), "pass", "none")
}
