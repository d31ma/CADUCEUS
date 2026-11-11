package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
)

type clientDohResponse struct {
	Status int               `json:"Status"`
	Answer []clientDohAnswer `json:"Answer"`
}

type clientDohAnswer struct {
	Type int    `json:"type"`
	Data string `json:"data"`
}

func devDnsSettings() DevDnsSettings {
	return DevDnsSettings{Domains: []string{"example.test", "mesail.localhost"}, MailHost: "mx.mesail.localhost", Spf: "mx"}
}

func devDnsHandler(t *testing.T, dns DevDnsSettings) http.Handler {
	t.Helper()
	settings := nativeSettings(t, "http://127.0.0.1:1")
	settings.DevDns = dns
	return handlerFor(t, settings)
}

func askDevDns(t *testing.T, handler http.Handler, name string, kind string) (*httptest.ResponseRecorder, DevDnsResponse) {
	t.Helper()
	query := url.Values{"name": {name}, "type": {kind}}
	reply := call(t, handler, http.MethodGet, "/_dev/dns-query?"+query.Encode(), "")
	var response DevDnsResponse
	if err := json.Unmarshal(reply.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode %q: %v", reply.Body.String(), err)
	}
	return reply, response
}

func TestDevDnsAnswersTheConfiguredDomains(t *testing.T) {
	handler := devDnsHandler(t, devDnsSettings())
	cases := []struct {
		name     string
		kind     string
		status   int
		answered string
		data     string
		number   int
	}{
		{name: "example.test", kind: "MX", status: 0, answered: "example.test.", data: "10 mx.mesail.localhost.", number: 15},
		{name: "Example.Test.", kind: "mx", status: 0, answered: "example.test.", data: "10 mx.mesail.localhost.", number: 15},
		{name: "example.test", kind: "TXT", status: 0, answered: "example.test.", data: `"v=spf1 mx -all"`, number: 16},
		{name: "mesail._domainkey.example.test", kind: "TXT", status: 0, answered: "mesail._domainkey.example.test.", data: `"v=DKIM1; k=rsa; p=bWVzYWlsLWRldi1kbnMtcGxhY2Vob2xkZXIta2V5"`, number: 16},
		{name: "other._domainkey.mesail.localhost", kind: "TXT", status: 0, answered: "other._domainkey.mesail.localhost.", data: `"v=DKIM1; k=rsa; p=bWVzYWlsLWRldi1kbnMtcGxhY2Vob2xkZXIta2V5"`, number: 16},
		{name: "_dmarc.example.test", kind: "TXT", status: 0, answered: "_dmarc.example.test.", data: `"v=DMARC1; p=reject"`, number: 16},
		{name: "example.test", kind: "A", status: 3},
		{name: "_dmarc.example.test", kind: "MX", status: 3},
		{name: "unknown.test", kind: "MX", status: 3},
		{name: "mesail._domainkey.unknown.test", kind: "TXT", status: 3},
		{name: "a.b._domainkey.example.test", kind: "TXT", status: 3},
		{name: "", kind: "", status: 3},
	}
	for _, test := range cases {
		reply, response := askDevDns(t, handler, test.name, test.kind)
		want := DevDnsResponse{Status: test.status}
		if test.data != "" {
			want.Answer = []DevDnsAnswer{{Name: test.answered, Type: test.number, TTL: 60, Data: test.data}}
		}
		if reply.Code != http.StatusOK || reply.Header().Get("Content-Type") != "application/dns-json" || !slices.Equal(response.Answer, want.Answer) || response.Status != want.Status {
			t.Errorf("%s %s: got %d %q %+v, want %+v", test.name, test.kind, reply.Code, reply.Header().Get("Content-Type"), response, want)
		}
	}
}

func TestDevDnsMissingAnswerLeavesOutTheAnswerList(t *testing.T) {
	reply, _ := askDevDns(t, devDnsHandler(t, devDnsSettings()), "unknown.test", "MX")
	if reply.Body.String() != `{"Status":3}` {
		t.Fatalf("got %s", reply.Body.String())
	}
}

func TestDevDnsAllowsOnlyGet(t *testing.T) {
	reply := call(t, devDnsHandler(t, devDnsSettings()), http.MethodPost, "/_dev/dns-query?name=example.test&type=MX", "")
	if reply.Code != http.StatusMethodNotAllowed || !strings.Contains(reply.Body.String(), "only GET") {
		t.Fatalf("got %d %s", reply.Code, reply.Body.String())
	}
}

func TestDevDnsIsMissingWhenOff(t *testing.T) {
	reply := call(t, devDnsHandler(t, DevDnsSettings{}), http.MethodGet, "/_dev/dns-query?name=example.test&type=MX", "")
	if reply.Code != http.StatusNotFound || !strings.Contains(reply.Body.String(), "not-found") {
		t.Fatalf("got %d %s", reply.Code, reply.Body.String())
	}
}

func TestDevDnsOnlyAnswersTheExactPath(t *testing.T) {
	reply := call(t, devDnsHandler(t, devDnsSettings()), http.MethodGet, "/_dev/dns-query/more?name=example.test&type=MX", "")
	if reply.Header().Get("Content-Type") == "application/dns-json" || reply.Body.String() != "web home" {
		t.Fatalf("got %d %s", reply.Code, reply.Body.String())
	}
}

func TestDevDnsAnswersPassTheDirectoryChecks(t *testing.T) {
	server := httptest.NewServer(devDnsHandler(t, devDnsSettings()))
	t.Cleanup(server.Close)
	endpoint := server.URL + "/_dev/dns-query"
	mailHosts := clientLookup(t, endpoint, "example.test", "MX", 15)
	domainTexts := clientLookup(t, endpoint, "example.test", "TXT", 16)
	dkimTexts := clientLookup(t, endpoint, "mesail._domainkey.example.test", "TXT", 16)
	dmarcTexts := clientLookup(t, endpoint, "_dmarc.example.test", "TXT", 16)
	missing := clientLookup(t, endpoint, "_dmarc.unknown.test", "TXT", 16)
	mxHost := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(clientMxHost(onlyOne(t, mailHosts)))), ".")
	spf := clientText(onlyOne(t, domainTexts))
	dkim := clientTags(clientText(onlyOne(t, dkimTexts)))
	dmarc := clientText(onlyOne(t, dmarcTexts))
	dmarcTags := clientTags(dmarc)
	if mxHost != "mx.mesail.localhost" {
		t.Errorf("mx %q", mailHosts)
	}
	if !strings.HasPrefix(spf, "v=spf1") || !slices.Contains(strings.Fields(spf), "mx") {
		t.Errorf("spf %q", domainTexts)
	}
	if dkim["v"] != "DKIM1" || dkim["p"] == "" {
		t.Errorf("dkim %q", dkimTexts)
	}
	if !strings.HasPrefix(dmarc, "v=DMARC1;") || !slices.Contains([]string{"none", "quarantine", "reject"}, dmarcTags["p"]) {
		t.Errorf("dmarc %q", dmarcTexts)
	}
	if len(missing) != 0 {
		t.Errorf("missing %q", missing)
	}
}

func clientLookup(t *testing.T, endpoint string, name string, kind string, number int) []string {
	t.Helper()
	address, err := url.Parse(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	values := address.Query()
	values.Set("name", name)
	values.Set("type", kind)
	address.RawQuery = values.Encode()
	request, err := http.NewRequest(http.MethodGet, address.String(), nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Accept", "application/dns-json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var parsed clientDohResponse
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&parsed); err != nil || response.StatusCode != http.StatusOK || (parsed.Status != 0 && parsed.Status != 3) {
		t.Fatalf("%s %s: %d %+v %v", name, kind, response.StatusCode, parsed, err)
	}
	found := []string{}
	for _, answer := range parsed.Answer {
		if answer.Type == number {
			found = append(found, answer.Data)
		}
	}
	return found
}

func onlyOne(t *testing.T, values []string) string {
	t.Helper()
	if len(values) != 1 {
		t.Fatalf("want one value, got %q", values)
	}
	return values[0]
}

func clientMxHost(raw string) string {
	_, host, _ := strings.Cut(strings.TrimSpace(raw), " ")
	return host
}

func clientText(raw string) string {
	inner := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(raw), `"`), `"`)
	return strings.NewReplacer(`\"`, `"`, `\\`, `\`).Replace(strings.ReplaceAll(inner, `" "`, ""))
}

func clientTags(record string) map[string]string {
	tags := map[string]string{}
	for _, piece := range strings.Split(record, ";") {
		name, value, _ := strings.Cut(piece, "=")
		tags[strings.TrimSpace(name)] = strings.TrimSpace(value)
	}
	return tags
}

func TestDevDnsSettingsAreReadWhenSet(t *testing.T) {
	values := map[string]string{
		"MESAIL_ROUTER_UPSTREAMS":         "identity=http://127.0.0.1:8101",
		"MESAIL_ROUTER_DEV_DNS_DOMAINS":   "example.test, mesail.localhost",
		"MESAIL_ROUTER_DEV_DNS_MAIL_HOST": "mx.mesail.localhost",
		"MESAIL_ROUTER_DEV_DNS_SPF":       "include:_spf.mesail.localhost",
	}
	settings, err := SettingsReader{Lookup: lookupIn(values)}.Read()
	want := DevDnsSettings{Domains: []string{"example.test", "mesail.localhost"}, MailHost: "mx.mesail.localhost", Spf: "include:_spf.mesail.localhost"}
	if err != nil || !slices.Equal(settings.DevDns.Domains, want.Domains) || settings.DevDns.MailHost != want.MailHost || settings.DevDns.Spf != want.Spf || !settings.DevDns.On() {
		t.Fatalf("got %+v, %v", settings.DevDns, err)
	}
}

func TestDevDnsSettingsAreOffByDefault(t *testing.T) {
	values := map[string]string{"MESAIL_ROUTER_UPSTREAMS": "identity=http://127.0.0.1:8101", "MESAIL_ROUTER_DEV_DNS_MAIL_HOST": "Not A Host"}
	settings, err := SettingsReader{Lookup: lookupIn(values)}.Read()
	if err != nil || settings.DevDns.On() {
		t.Fatalf("got %+v, %v", settings.DevDns, err)
	}
}

func TestDevDnsSettingsRefuseBadValues(t *testing.T) {
	cases := []struct {
		values  map[string]string
		setting string
	}{
		{values: map[string]string{"MESAIL_ROUTER_DEV_DNS_DOMAINS": "example.test", "MESAIL_ROUTER_DEV_DNS_SPF": "mx"}, setting: "MESAIL_ROUTER_DEV_DNS_MAIL_HOST"},
		{values: map[string]string{"MESAIL_ROUTER_DEV_DNS_DOMAINS": "example.test", "MESAIL_ROUTER_DEV_DNS_MAIL_HOST": "mx.example.test"}, setting: "MESAIL_ROUTER_DEV_DNS_SPF"},
		{values: map[string]string{"MESAIL_ROUTER_DEV_DNS_DOMAINS": "example.test", "MESAIL_ROUTER_DEV_DNS_MAIL_HOST": "mx.example.test.", "MESAIL_ROUTER_DEV_DNS_SPF": "mx"}, setting: "MESAIL_ROUTER_DEV_DNS_MAIL_HOST"},
		{values: map[string]string{"MESAIL_ROUTER_DEV_DNS_DOMAINS": "example.test", "MESAIL_ROUTER_DEV_DNS_MAIL_HOST": "mx.example.test", "MESAIL_ROUTER_DEV_DNS_SPF": `mx "a"`}, setting: "MESAIL_ROUTER_DEV_DNS_SPF"},
		{values: map[string]string{"MESAIL_ROUTER_DEV_DNS_DOMAINS": "Example.test", "MESAIL_ROUTER_DEV_DNS_MAIL_HOST": "mx.example.test", "MESAIL_ROUTER_DEV_DNS_SPF": "mx"}, setting: "MESAIL_ROUTER_DEV_DNS_DOMAINS"},
		{values: map[string]string{"MESAIL_ROUTER_DEV_DNS_DOMAINS": "localhost", "MESAIL_ROUTER_DEV_DNS_MAIL_HOST": "mx.example.test", "MESAIL_ROUTER_DEV_DNS_SPF": "mx"}, setting: "MESAIL_ROUTER_DEV_DNS_DOMAINS"},
		{values: map[string]string{"MESAIL_ROUTER_DEV_DNS_DOMAINS": "-bad.test", "MESAIL_ROUTER_DEV_DNS_MAIL_HOST": "mx.example.test", "MESAIL_ROUTER_DEV_DNS_SPF": "mx"}, setting: "MESAIL_ROUTER_DEV_DNS_DOMAINS"},
	}
	for _, test := range cases {
		test.values["MESAIL_ROUTER_UPSTREAMS"] = "identity=http://127.0.0.1:8101"
		_, err := SettingsReader{Lookup: lookupIn(test.values)}.Read()
		var invalid InvalidSetting
		if !errors.As(err, &invalid) || invalid.Name != test.setting {
			t.Errorf("%v: got %v, want %s", test.values, err, test.setting)
		}
	}
}
