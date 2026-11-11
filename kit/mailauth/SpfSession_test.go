package mailauth

import (
	"context"
	"net"
	"testing"
)

func spfInput(mailFrom string) Input {
	return Input{RemoteIp: net.ParseIP("192.0.2.10"), Helo: "mail.outside.test", MailFrom: mailFrom, Raw: []byte("From: a@outside.test\r\n\r\nHi\r\n")}
}

func spfCheck(fake *FakeResolver, input Input) Results {
	return NewChecker(fake).Check(context.Background(), input)
}

func expectSpf(t *testing.T, results Results, want string) {
	t.Helper()
	if results.Spf != want {
		t.Fatalf("spf: got %q, want %q", results.Spf, want)
	}
}

func TestSpfPassesForAListedNetwork(t *testing.T) {
	fake := newFakeResolver().text("outside.test", "v=spf1 ip4:192.0.2.0/24 -all")
	results := spfCheck(fake, spfInput("user@outside.test"))
	expectSpf(t, results, "pass")
	if results.SpfDomain != "outside.test" {
		t.Fatalf("got %q", results.SpfDomain)
	}
}

func TestSpfFailsOnDashAll(t *testing.T) {
	fake := newFakeResolver().text("outside.test", "v=spf1 ip4:198.51.100.1 -all")
	expectSpf(t, spfCheck(fake, spfInput("user@outside.test")), "fail")
}

func TestSpfSoftFailsOnTildeAll(t *testing.T) {
	fake := newFakeResolver().text("outside.test", "v=spf1 ~all")
	expectSpf(t, spfCheck(fake, spfInput("user@outside.test")), "softfail")
}

func TestSpfIsNeutralOnQuestionAll(t *testing.T) {
	fake := newFakeResolver().text("outside.test", "v=spf1 ?all")
	expectSpf(t, spfCheck(fake, spfInput("user@outside.test")), "neutral")
}

func TestSpfIsNeutralWhenNothingMatches(t *testing.T) {
	fake := newFakeResolver().text("outside.test", "v=spf1 ip4:198.51.100.1")
	expectSpf(t, spfCheck(fake, spfInput("user@outside.test")), "neutral")
}

func TestSpfIsNoneWithoutARecord(t *testing.T) {
	fake := newFakeResolver().text("outside.test", "some other text")
	expectSpf(t, spfCheck(fake, spfInput("user@outside.test")), "none")
}

func TestSpfIsNoneWithoutARemoteAddress(t *testing.T) {
	input := spfInput("user@outside.test")
	input.RemoteIp = nil
	expectSpf(t, spfCheck(newFakeResolver(), input), "none")
}

func TestSpfGivesPermErrorForTwoRecords(t *testing.T) {
	fake := newFakeResolver().text("outside.test", "v=spf1 -all", "v=spf1 +all")
	expectSpf(t, spfCheck(fake, spfInput("user@outside.test")), "permerror")
}

func TestSpfGivesPermErrorForAnUnknownMechanism(t *testing.T) {
	fake := newFakeResolver().text("outside.test", "v=spf1 foo:bar +all")
	expectSpf(t, spfCheck(fake, spfInput("user@outside.test")), "permerror")
}

func TestSpfIgnoresUnknownModifiersAndExplanation(t *testing.T) {
	fake := newFakeResolver().text("outside.test", "v=spf1 foo=bar exp=explain.%{d} +all")
	expectSpf(t, spfCheck(fake, spfInput("user@outside.test")), "pass")
}

func TestSpfGivesPermErrorForBadSyntax(t *testing.T) {
	records := []string{
		"v=spf1 all:x", "v=spf1 include", "v=spf1 a:", "v=spf1 a/33", "v=spf1 a//129", "v=spf1 ip4:300.1.1.1",
		"v=spf1 ip4:2001:db8::1", "v=spf1 ip6:192.0.2.1", "v=spf1 redirect=a.test redirect=b.test", "v=spf1 a=x",
	}
	for _, record := range records {
		fake := newFakeResolver().text("outside.test", record)
		if got := spfCheck(fake, spfInput("user@outside.test")).Spf; got != "permerror" {
			t.Errorf("%q: got %q", record, got)
		}
	}
}

func TestSpfPassesThroughAnInclude(t *testing.T) {
	fake := newFakeResolver().
		text("outside.test", "v=spf1 include:_spf.provider.test -all").
		text("_spf.provider.test", "v=spf1 a:mail.provider.test -all").
		address("mail.provider.test", "192.0.2.10")
	expectSpf(t, spfCheck(fake, spfInput("user@outside.test")), "pass")
}

func TestSpfIncludeThatFailsDoesNotMatch(t *testing.T) {
	fake := newFakeResolver().
		text("outside.test", "v=spf1 include:_spf.provider.test ~all").
		text("_spf.provider.test", "v=spf1 -all")
	expectSpf(t, spfCheck(fake, spfInput("user@outside.test")), "softfail")
}

func TestSpfIncludeWithoutARecordIsPermError(t *testing.T) {
	fake := newFakeResolver().text("outside.test", "v=spf1 include:_spf.missing.test -all")
	expectSpf(t, spfCheck(fake, spfInput("user@outside.test")), "permerror")
}

func TestSpfFollowsARedirect(t *testing.T) {
	fake := newFakeResolver().
		text("outside.test", "v=spf1 redirect=_spf.provider.test").
		text("_spf.provider.test", "v=spf1 ip4:192.0.2.10 -all")
	expectSpf(t, spfCheck(fake, spfInput("user@outside.test")), "pass")
}

func TestSpfRedirectToAMissingRecordIsPermError(t *testing.T) {
	fake := newFakeResolver().text("outside.test", "v=spf1 redirect=_spf.missing.test")
	expectSpf(t, spfCheck(fake, spfInput("user@outside.test")), "permerror")
}

func TestSpfMatchesMxHostsWithAPrefix(t *testing.T) {
	fake := newFakeResolver().
		text("outside.test", "v=spf1 mx/24 -all").
		exchange("outside.test", "mx1.outside.test").
		address("mx1.outside.test", "192.0.2.200")
	expectSpf(t, spfCheck(fake, spfInput("user@outside.test")), "pass")
}

func TestSpfMatchesIp6Networks(t *testing.T) {
	fake := newFakeResolver().text("outside.test", "v=spf1 a//64 ip6:2001:db8::/32 -all").address("outside.test", "2001:db9::1")
	input := spfInput("user@outside.test")
	input.RemoteIp = net.ParseIP("2001:db8::25")
	expectSpf(t, spfCheck(fake, input), "pass")
}

func TestSpfPointerNeverMatches(t *testing.T) {
	fake := newFakeResolver().text("outside.test", "v=spf1 ptr ptr:outside.test -all")
	expectSpf(t, spfCheck(fake, spfInput("user@outside.test")), "fail")
}

func TestSpfStopsAfterTenLookups(t *testing.T) {
	fake := newFakeResolver().text("outside.test", "v=spf1 a:a.test a:a.test a:a.test a:a.test a:a.test a:a.test a:a.test a:a.test a:a.test a:a.test a:a.test +all").
		address("a.test", "127.0.0.9")
	expectSpf(t, spfCheck(fake, spfInput("user@outside.test")), "permerror")
}

func TestSpfAllowsTenLookups(t *testing.T) {
	fake := newFakeResolver().text("outside.test", "v=spf1 a:a.test a:a.test a:a.test a:a.test a:a.test a:a.test a:a.test a:a.test a:a.test a:a.test +all").
		address("a.test", "127.0.0.9")
	expectSpf(t, spfCheck(fake, spfInput("user@outside.test")), "pass")
}

func TestSpfStopsAfterTwoVoidLookups(t *testing.T) {
	fake := newFakeResolver().text("outside.test", "v=spf1 a:void1.test mx:void2.test a:void3.test +all")
	expectSpf(t, spfCheck(fake, spfInput("user@outside.test")), "permerror")
}

func TestSpfAllowsTwoVoidLookups(t *testing.T) {
	fake := newFakeResolver().text("outside.test", "v=spf1 a:void1.test exists:void2.test +all")
	expectSpf(t, spfCheck(fake, spfInput("user@outside.test")), "pass")
}

func TestSpfRefusesTooManyMxHosts(t *testing.T) {
	fake := newFakeResolver().text("outside.test", "v=spf1 mx +all").
		exchange("outside.test", "1.test", "2.test", "3.test", "4.test", "5.test", "6.test", "7.test", "8.test", "9.test", "10.test", "11.test")
	expectSpf(t, spfCheck(fake, spfInput("user@outside.test")), "permerror")
}

func TestSpfGivesTempErrorWhenDnsFails(t *testing.T) {
	fake := newFakeResolver().fail("outside.test")
	expectSpf(t, spfCheck(fake, spfInput("user@outside.test")), "temperror")
}

func TestSpfGivesTempErrorWhenAnIncludeFails(t *testing.T) {
	fake := newFakeResolver().text("outside.test", "v=spf1 include:broken.test -all").fail("broken.test")
	expectSpf(t, spfCheck(fake, spfInput("user@outside.test")), "temperror")
}

func TestSpfGivesTempErrorWhenAnMxHostFails(t *testing.T) {
	fake := newFakeResolver().text("outside.test", "v=spf1 mx -all").exchange("outside.test", "mx.broken.test").fail("mx.broken.test")
	expectSpf(t, spfCheck(fake, spfInput("user@outside.test")), "temperror")
}

func TestSpfGivesTempErrorWhenAnAddressLookupFails(t *testing.T) {
	fake := newFakeResolver().text("outside.test", "v=spf1 a:broken.test exists:broken.test -all").fail("broken.test")
	expectSpf(t, spfCheck(fake, spfInput("user@outside.test")), "temperror")
}

func TestSpfGivesTempErrorWhenMxLookupFails(t *testing.T) {
	fake := newFakeResolver().text("outside.test", "v=spf1 mx:broken.test -all").fail("broken.test")
	expectSpf(t, spfCheck(fake, spfInput("user@outside.test")), "temperror")
}

func TestSpfExpandsMacros(t *testing.T) {
	fake := newFakeResolver().
		text("outside.test", "v=spf1 exists:%{i}.%{l}.%{o}._spf.%{d} -all").
		address("192.0.2.10.user.outside.test._spf.outside.test", "127.0.0.2")
	expectSpf(t, spfCheck(fake, spfInput("user@outside.test")), "pass")
}

func TestSpfExpandsSenderHeloAndEscapes(t *testing.T) {
	fake := newFakeResolver().
		text("outside.test", "v=spf1 exists:%{s}.%{h}.x%%y%-z%_w.test -all").
		address("user@outside.test.mail.outside.test.x%y%20z w.test", "127.0.0.2")
	expectSpf(t, spfCheck(fake, spfInput("user@outside.test")), "pass")
}

func TestSpfExpandsIp6AsNibbles(t *testing.T) {
	fake := newFakeResolver().
		text("outside.test", "v=spf1 exists:%{i}.ip6.test -all").
		address("2.0.0.1.0.d.b.8.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.1.ip6.test", "127.0.0.2")
	input := spfInput("user@outside.test")
	input.RemoteIp = net.ParseIP("2001:db8::1")
	expectSpf(t, spfCheck(fake, input), "pass")
}

func TestSpfRefusesUnsupportedMacros(t *testing.T) {
	for _, record := range []string{"v=spf1 exists:%{ir}.test -all", "v=spf1 exists:%{v}.test -all", "v=spf1 exists:%x.test -all", "v=spf1 include:%{d}. -all", "v=spf1 exists:. -all"} {
		fake := newFakeResolver().text("outside.test", record)
		if got := spfCheck(fake, spfInput("user@outside.test")).Spf; got != "permerror" {
			t.Errorf("%q: got %q", record, got)
		}
	}
}

func TestSpfChecksTheHeloDomainForAnEmptyMailFrom(t *testing.T) {
	fake := newFakeResolver().text("mail.outside.test", "v=spf1 exists:%{l}.%{o}.test -all").address("postmaster.mail.outside.test.test", "127.0.0.2")
	results := spfCheck(fake, spfInput(""))
	expectSpf(t, results, "pass")
	if results.SpfDomain != "mail.outside.test" {
		t.Fatalf("got %q", results.SpfDomain)
	}
}

func TestSpfUsesPostmasterForAMailFromWithoutLocalPart(t *testing.T) {
	fake := newFakeResolver().text("outside.test", "v=spf1 exists:%{l}.test -all").address("postmaster.test", "127.0.0.2")
	expectSpf(t, spfCheck(fake, spfInput("<Outside.Test.>")), "pass")
}
