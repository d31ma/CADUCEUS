package mailauth

import (
	"context"
	"encoding/json"
	"net"
	"net/netip"
	"os"
	"strings"
	"testing"
	"time"
)

// fakeDNS answers from the golden DNS map: "TYPE name" -> records.
type fakeDNS map[string]json.RawMessage

func (dns fakeDNS) lookup(kind, name string) (json.RawMessage, error) {
	value, ok := dns[kind+" "+strings.TrimSuffix(name, ".")]
	if !ok || string(value) == "[]" {
		return nil, &DNSError{Name: name, NotFound: true, Message: "ENOTFOUND " + name}
	}
	return value, nil
}

func (dns fakeDNS) LookupTXT(_ context.Context, name string) ([]string, error) {
	value, err := dns.lookup("TXT", name)
	if err != nil {
		return nil, err
	}
	var rows [][]string
	if err := json.Unmarshal(value, &rows); err != nil {
		return nil, err
	}
	var out []string
	for _, row := range rows {
		out = append(out, strings.Join(row, ""))
	}
	return out, nil
}

func (dns fakeDNS) LookupIP(_ context.Context, network, host string) ([]netip.Addr, error) {
	kind := "A"
	if network == "ip6" {
		kind = "AAAA"
	}
	value, err := dns.lookup(kind, host)
	if err != nil {
		return nil, err
	}
	var rows []string
	_ = json.Unmarshal(value, &rows)
	var out []netip.Addr
	for _, row := range rows {
		out = append(out, netip.MustParseAddr(row))
	}
	return out, nil
}

func (dns fakeDNS) LookupMX(_ context.Context, name string) ([]*net.MX, error) {
	value, err := dns.lookup("MX", name)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		Exchange string `json:"exchange"`
		Priority uint16 `json:"priority"`
	}
	_ = json.Unmarshal(value, &rows)
	var out []*net.MX
	for _, row := range rows {
		out = append(out, &net.MX{Host: row.Exchange, Pref: row.Priority})
	}
	return out, nil
}

func (dns fakeDNS) LookupAddr(_ context.Context, address string) ([]string, error) {
	return nil, &DNSError{NotFound: true, Message: "ENOTFOUND"}
}

// TestMatchesMailauth replays the cases recorded with the mailauth npm
// package (testdata/mailauth-golden.cjs) and requires identical headers.
func TestMatchesMailauth(t *testing.T) {
	encoded, err := os.ReadFile("testdata/mailauth-golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		DNS   fakeDNS `json:"dns"`
		Cases []struct {
			Name    string `json:"name"`
			Raw     string `json:"raw"`
			Headers string `json:"headers"`
			Options struct {
				IP     string `json:"ip"`
				Helo   string `json:"helo"`
				Sender string `json:"sender"`
			} `json:"options"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(encoded, &golden); err != nil {
		t.Fatal(err)
	}
	now := func() time.Time { return time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC) }
	for _, c := range golden.Cases {
		t.Run(c.Name, func(t *testing.T) {
			result, err := Authenticate(context.Background(), []byte(c.Raw), Options{
				IP: c.Options.IP, Helo: c.Options.Helo, Sender: c.Options.Sender, MTA: "mx.test", Resolver: golden.DNS, Now: now,
			})
			if err != nil {
				t.Fatal(err)
			}
			if result.Headers != c.Headers {
				t.Fatalf("headers:\n%s\nwant:\n%s", result.Headers, c.Headers)
			}
		})
	}
}

func TestOrganizationalDomain(t *testing.T) {
	for domain, want := range map[string]string{
		"mail.example.com": "example.com", "example.com": "example.com", "a.b.example.co.uk": "example.co.uk",
		"co.uk": "", "com": "", "user.github.io": "user.github.io", "Example.COM.": "example.com",
	} {
		if got := OrganizationalDomain(domain); got != want {
			t.Fatalf("%s = %q, want %q", domain, got, want)
		}
	}
}

func TestFoldLines(t *testing.T) {
	line := "spf=softfail (" + strings.Repeat("x", 140) + ") smtp.mailfrom=a@b smtp.helo=c"
	got := foldLines(line, 160)
	lines := strings.Split(got, "\r\n")
	if len(lines) != 2 || len(lines[0]) > 160 || lines[1] != " smtp.mailfrom=a@b smtp.helo=c" {
		t.Fatalf("folded = %q", got)
	}
}
