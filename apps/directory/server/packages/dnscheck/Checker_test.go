package dnscheck

import (
	"errors"
	"testing"
)

type fakeLookup struct {
	mx       map[string][]string
	txt      map[string][]string
	failures map[string]error
}

func (fake fakeLookup) Mx(name string) ([]string, error) {
	return fake.mx[name], fake.failures["mx:"+name]
}

func (fake fakeLookup) Txt(name string) ([]string, error) {
	return fake.txt[name], fake.failures["txt:"+name]
}

func expected() Expected {
	return Expected{MailHost: "mx.mesail.example", SpfMechanism: "include:_spf.mesail.example", DkimSelector: "mesail"}
}

func goodLookup() fakeLookup {
	return fakeLookup{
		mx: map[string][]string{"example.com": {"mx.mesail.example"}},
		txt: map[string][]string{
			"example.com":                   {"google-site-verification=abc", "v=spf1 include:_spf.mesail.example -all"},
			"mesail._domainkey.example.com": {"v=DKIM1; k=rsa; p=MIGfMA0"},
			"_dmarc.example.com":            {"v=DMARC1; p=reject; rua=mailto:d@example.com"},
		},
		failures: map[string]error{},
	}
}

func check(t *testing.T, fake fakeLookup) States {
	t.Helper()
	states, err := NewChecker(fake).Check("example.com", expected())
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	return states
}

func TestAllRecordsCorrectAreOk(t *testing.T) {
	got := check(t, goodLookup())
	want := States{Mx: "ok", Spf: "ok", Dkim: "ok", Dmarc: "ok"}
	if got != want {
		t.Fatalf("got %+v", got)
	}
}

func TestMxMatchesIgnoringCaseAndTrailingDot(t *testing.T) {
	fake := goodLookup()
	fake.mx["example.com"] = []string{"backup.example.net", "MX.Mesail.Example."}
	if got := check(t, fake).Mx; got != (StateNames{}).Ok() {
		t.Fatalf("got %q", got)
	}
}

func TestMxWithOtherHostsIsWrong(t *testing.T) {
	fake := goodLookup()
	fake.mx["example.com"] = []string{"mx.other.example"}
	if got := check(t, fake).Mx; got != (StateNames{}).Wrong() {
		t.Fatalf("got %q", got)
	}
}

func TestMxWithoutRecordsIsMissing(t *testing.T) {
	fake := goodLookup()
	delete(fake.mx, "example.com")
	if got := check(t, fake).Mx; got != (StateNames{}).Missing() {
		t.Fatalf("got %q", got)
	}
}

func TestSpfWithoutRecordIsMissing(t *testing.T) {
	fake := goodLookup()
	fake.txt["example.com"] = []string{"google-site-verification=abc"}
	if got := check(t, fake).Spf; got != (StateNames{}).Missing() {
		t.Fatalf("got %q", got)
	}
}

func TestSpfPrefixIgnoresCase(t *testing.T) {
	fake := goodLookup()
	fake.txt["example.com"] = []string{"V=SPF1 include:_spf.mesail.example ~all"}
	if got := check(t, fake).Spf; got != (StateNames{}).Ok() {
		t.Fatalf("got %q", got)
	}
}

func TestSpfWithTwoRecordsIsWrong(t *testing.T) {
	fake := goodLookup()
	fake.txt["example.com"] = []string{"v=spf1 include:_spf.mesail.example -all", "v=spf1 mx -all"}
	if got := check(t, fake).Spf; got != (StateNames{}).Wrong() {
		t.Fatalf("got %q", got)
	}
}

func TestSpfWithoutTheMechanismIsWrong(t *testing.T) {
	fake := goodLookup()
	fake.txt["example.com"] = []string{"v=spf1 mx -all"}
	if got := check(t, fake).Spf; got != (StateNames{}).Wrong() {
		t.Fatalf("got %q", got)
	}
}

func TestSpfMechanismMustBeAWholeTerm(t *testing.T) {
	fake := goodLookup()
	fake.txt["example.com"] = []string{"v=spf1 include:_spf.mesail.example.evil -all"}
	if got := check(t, fake).Spf; got != (StateNames{}).Wrong() {
		t.Fatalf("got %q", got)
	}
}

func TestDkimWithoutRecordIsMissing(t *testing.T) {
	fake := goodLookup()
	delete(fake.txt, "mesail._domainkey.example.com")
	if got := check(t, fake).Dkim; got != (StateNames{}).Missing() {
		t.Fatalf("got %q", got)
	}
}

func TestDkimWithSpacesAroundTagsIsOk(t *testing.T) {
	fake := goodLookup()
	fake.txt["mesail._domainkey.example.com"] = []string{" v = DKIM1 ;k=rsa;  p = MIGfMA0 ; "}
	if got := check(t, fake).Dkim; got != (StateNames{}).Ok() {
		t.Fatalf("got %q", got)
	}
}

func TestDkimWithEmptyKeyIsWrong(t *testing.T) {
	fake := goodLookup()
	fake.txt["mesail._domainkey.example.com"] = []string{"v=DKIM1; k=rsa; p="}
	if got := check(t, fake).Dkim; got != (StateNames{}).Wrong() {
		t.Fatalf("got %q", got)
	}
}

func TestDkimWithoutVersionIsWrong(t *testing.T) {
	fake := goodLookup()
	fake.txt["mesail._domainkey.example.com"] = []string{"k=rsa; p=MIGfMA0"}
	if got := check(t, fake).Dkim; got != (StateNames{}).Wrong() {
		t.Fatalf("got %q", got)
	}
}

func TestDmarcWithoutRecordIsMissing(t *testing.T) {
	fake := goodLookup()
	delete(fake.txt, "_dmarc.example.com")
	if got := check(t, fake).Dmarc; got != (StateNames{}).Missing() {
		t.Fatalf("got %q", got)
	}
}

func TestDmarcWithPolicyNoneIsOk(t *testing.T) {
	fake := goodLookup()
	fake.txt["_dmarc.example.com"] = []string{"v=DMARC1; p=none"}
	if got := check(t, fake).Dmarc; got != (StateNames{}).Ok() {
		t.Fatalf("got %q", got)
	}
}

func TestDmarcWithPolicyQuarantineIsOk(t *testing.T) {
	fake := goodLookup()
	fake.txt["_dmarc.example.com"] = []string{"v=DMARC1;p=quarantine"}
	if got := check(t, fake).Dmarc; got != (StateNames{}).Ok() {
		t.Fatalf("got %q", got)
	}
}

func TestDmarcWithUnknownPolicyIsWrong(t *testing.T) {
	fake := goodLookup()
	fake.txt["_dmarc.example.com"] = []string{"v=DMARC1; p=monitor"}
	if got := check(t, fake).Dmarc; got != (StateNames{}).Wrong() {
		t.Fatalf("got %q", got)
	}
}

func TestDmarcWithoutPolicyIsWrong(t *testing.T) {
	fake := goodLookup()
	fake.txt["_dmarc.example.com"] = []string{"v=DMARC1; rua=mailto:d@example.com"}
	if got := check(t, fake).Dmarc; got != (StateNames{}).Wrong() {
		t.Fatalf("got %q", got)
	}
}

func TestDmarcNotStartingWithVersionIsWrong(t *testing.T) {
	fake := goodLookup()
	fake.txt["_dmarc.example.com"] = []string{"p=reject; v=DMARC1"}
	if got := check(t, fake).Dmarc; got != (StateNames{}).Wrong() {
		t.Fatalf("got %q", got)
	}
}

func lookupError(t *testing.T, key string) error {
	t.Helper()
	fake := goodLookup()
	fake.failures[key] = errors.New("resolver down")
	_, err := NewChecker(fake).Check("example.com", expected())
	return err
}

func TestMxLookupErrorIsReturned(t *testing.T) {
	if err := lookupError(t, "mx:example.com"); err == nil {
		t.Fatal("expected an error")
	}
}

func TestSpfLookupErrorIsReturned(t *testing.T) {
	if err := lookupError(t, "txt:example.com"); err == nil {
		t.Fatal("expected an error")
	}
}

func TestDkimLookupErrorIsReturned(t *testing.T) {
	if err := lookupError(t, "txt:mesail._domainkey.example.com"); err == nil {
		t.Fatal("expected an error")
	}
}

func TestDmarcLookupErrorIsReturned(t *testing.T) {
	if err := lookupError(t, "txt:_dmarc.example.com"); err == nil {
		t.Fatal("expected an error")
	}
}
