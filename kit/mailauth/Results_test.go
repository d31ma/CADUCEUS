package mailauth

import (
	"strings"
	"testing"
)

func TestHeaderFieldFormatsTheResults(t *testing.T) {
	results := Results{Spf: "pass", SpfDomain: "outside.test", Dkim: []DkimResult{{Result: "pass", Domain: "outside.test", Selector: "s1"}}, Dmarc: "pass", FromDomain: "outside.test"}
	want := "Authentication-Results: mx.example.com; spf=pass smtp.mailfrom=outside.test;\r\n\tdkim=pass header.d=outside.test header.s=s1; dmarc=pass\r\n\theader.from=outside.test\r\n"
	if got := results.HeaderField("mx.example.com"); got != want {
		t.Fatalf("got %q", got)
	}
}

func TestHeaderFieldSaysNoneWithoutSignatures(t *testing.T) {
	results := Results{Spf: "none", Dmarc: "none"}
	want := "Authentication-Results: mx.example.com; spf=none; dkim=none; dmarc=none\r\n"
	if got := results.HeaderField("mx.example.com"); got != want {
		t.Fatalf("got %q", got)
	}
}

func TestHeaderFieldRemovesUnsafeCharacters(t *testing.T) {
	results := Results{Spf: "pass", SpfDomain: "evil.test;\r\nX-Injected: yes", Dkim: []DkimResult{{Result: "fail", Domain: "a (b) \"c\"", Selector: ";\r\n"}}, Dmarc: "fail", FromDomain: "x.test"}
	got := results.HeaderField("mx.example.com;\r\nBcc: x")
	if strings.Contains(got, "X-Injected:") || strings.Contains(got, "\r\nBcc") || strings.Contains(got, "(") || strings.Count(got, ";") != 3 {
		t.Fatalf("got %q", got)
	}
	if !strings.Contains(got, "smtp.mailfrom=evil.testX-Injectedyes") || strings.Contains(got, "header.s=") {
		t.Fatalf("got %q", got)
	}
}

func TestHeaderFieldFoldsLongLines(t *testing.T) {
	results := Results{Spf: "pass", SpfDomain: "outside.test", Dmarc: "pass", FromDomain: "outside.test"}
	for range 6 {
		results.Dkim = append(results.Dkim, DkimResult{Result: "pass", Domain: "a-rather-long-signing-domain.example.test", Selector: "selector2026"})
	}
	got := results.HeaderField("mx.example.com")
	if !strings.HasSuffix(got, "\r\n") {
		t.Fatalf("got %q", got)
	}
	for _, line := range strings.Split(strings.TrimSuffix(got, "\r\n"), "\r\n") {
		if len(line) > 78 {
			t.Fatalf("line too long: %q", line)
		}
	}
	if strings.Count(got, "\r\n\t") < 6 {
		t.Fatalf("got %q", got)
	}
}

func TestDkimSummaryPrefersAPass(t *testing.T) {
	results := Results{Dkim: []DkimResult{{Result: "fail"}, {Result: "pass"}}}
	if results.DkimSummary() != "pass" {
		t.Fatalf("got %q", results.DkimSummary())
	}
}

func TestDkimSummaryUsesTheFirstResult(t *testing.T) {
	results := Results{Dkim: []DkimResult{{Result: "temperror"}, {Result: "fail"}}}
	if results.DkimSummary() != "temperror" {
		t.Fatalf("got %q", results.DkimSummary())
	}
}

func TestDkimSummaryIsNoneWithoutSignatures(t *testing.T) {
	if got := (Results{}).DkimSummary(); got != "none" {
		t.Fatalf("got %q", got)
	}
}
