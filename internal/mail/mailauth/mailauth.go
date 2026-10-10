// Package mailauth verifies SPF, DKIM and DMARC for a received message and
// writes the Received-SPF and Authentication-Results headers.
//
// It ports the parts of the `mailauth` npm package (v5) that
// background/InboundSmtpServer.js used through `authenticate`. Not
// reproduced: ARC chain validation and sealing (a message with ARC headers
// gets no `arc=` comment), BIMI VMC/SVG validation (BIMI is a DNS record
// check only), and the full Public Suffix List (OrganizationalDomain is a
// heuristic over common multi-label suffixes).
package mailauth

import (
	"context"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/d31ma/caduceus/internal/mail/inbound"
)

// Options describes the sending connection.
type Options struct {
	IP       string
	Helo     string
	Sender   string
	MTA      string
	Resolver Resolver
	Now      func() time.Time
}

// DMARCResult is the DMARC evaluation; nil when the message has not exactly
// one From address.
type DMARCResult struct {
	Status Status
	Domain string
	Policy string
	Info   string
	// DKIMUnderSized is the unsigned byte count of the aligned signature.
	DKIMUnderSized int
}

// Result is one authentication.
type Result struct {
	SPF   SPFResult
	DKIM  []DKIMResult
	DMARC *DMARCResult
	BIMI  string
	// Headers is the block to prepend: Received-SPF and
	// Authentication-Results, CRLF terminated.
	Headers string
}

// Authenticate verifies raw as mailauth's authenticate did.
func Authenticate(ctx context.Context, raw []byte, options Options) (*Result, error) {
	if options.Resolver == nil {
		options.Resolver = SystemResolver{}
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.MTA == "" {
		options.MTA = "localhost"
	}
	dkim, headerFrom := verifyDKIM(ctx, options.Resolver, raw, options.Now())
	helo := options.Helo
	if helo == "" && options.IP != "" {
		helo = options.IP
	}
	if _, err := netip.ParseAddr(helo); err == nil {
		helo = "[" + helo + "]"
	}
	spf := checkSPF(ctx, options.Resolver, options.Sender, options.IP, helo, options.MTA, options.Now)

	var entries []string
	for _, result := range dkim {
		entries = append(entries, foldLines(result.Info, 160))
	}
	entries = append(entries, foldLines(spf.Info, 160))

	result := &Result{SPF: spf, DKIM: dkim}
	var spfDomains []alignedDomain
	if spf.Status.Result == "pass" && spf.Domain != "" {
		spfDomains = append(spfDomains, alignedDomain{domain: spf.Domain})
	}
	var dkimDomains []alignedDomain
	for _, signature := range dkim {
		if signature.Status.Result == "pass" {
			dkimDomains = append(dkimDomains, alignedDomain{domain: signature.SigningDomain, underSized: signature.UnderSized})
		}
	}
	arc := ""
	if !hasHeader(raw, "arc-seal") {
		arc = "none"
	}
	result.DMARC = verifyDMARC(ctx, options.Resolver, headerFrom, spfDomains, dkimDomains, arc)
	if result.DMARC != nil {
		entries = append(entries, foldLines(result.DMARC.Info, 160))
		if bimi := checkBIMI(ctx, options.Resolver, result.DMARC, raw); bimi != "" {
			result.BIMI = bimi
			entries = append(entries, foldLines(bimi, 160))
		}
	}
	headers := []string{spf.Header, "Authentication-Results: " + options.MTA + ";\r\n " + strings.Join(entries, ";\r\n ")}
	result.Headers = strings.Join(headers, "\r\n") + "\r\n"
	return result, nil
}

func hasHeader(raw []byte, name string) bool {
	rows, _ := splitMessage(raw)
	for _, row := range rows {
		if row.key == name {
			return true
		}
	}
	return false
}

var dmarcVersion = regexp.MustCompile(`^[vV][ \t]*=[ \t]*DMARC1[ \t]*(?:;|$)`)

type dmarcRecord struct {
	tags  map[string]string
	pct   int
	isOrg bool
}

func dmarcRecords(ctx context.Context, resolver Resolver, domain string) ([]string, error) {
	records, err := resolver.LookupTXT(ctx, "_dmarc."+domain)
	if err != nil {
		if IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	var matching []string
	for _, record := range records {
		record = strings.TrimSpace(record)
		if dmarcVersion.MatchString(record) {
			matching = append(matching, record)
		}
	}
	return matching, nil
}

func lookupDMARC(ctx context.Context, resolver Resolver, domain string) (*dmarcRecord, error) {
	records, err := dmarcRecords(ctx, resolver, domain)
	if err != nil {
		return nil, err
	}
	isOrg := false
	if len(records) == 0 {
		if org := OrganizationalDomain(domain); org != "" && org != domain {
			records, err = dmarcRecords(ctx, resolver, org)
			if err != nil {
				return nil, err
			}
			isOrg = true
		}
	}
	if len(records) != 1 {
		return nil, nil
	}
	record := &dmarcRecord{tags: map[string]string{}, isOrg: isOrg}
	for _, entry := range strings.Split(records[0], ";") {
		entry = strings.TrimSpace(entry)
		equals := strings.Index(entry, "=")
		if entry == "" || equals <= 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(entry[:equals]))
		value := strings.TrimSpace(entry[equals+1:])
		switch key {
		case "pct", "ri":
			number, _ := strconv.Atoi(strings.TrimLeft(value, "+"))
			if key == "pct" {
				record.pct = number
			}
		case "p", "sp", "np", "adkim", "aspf", "fo", "rf", "t", "psd":
			value = strings.ToLower(value)
		}
		record.tags[key] = value
	}
	return record, nil
}

func verifyDMARC(ctx context.Context, resolver Resolver, headerFrom []string, spfDomains, dkimDomains []alignedDomain, arc string) *DMARCResult {
	if len(headerFrom) != 1 {
		return nil
	}
	from := headerFrom[0]
	domain := from
	if at := strings.Index(from, "@"); at >= 0 {
		domain = from[at+1:]
	}
	domain = strings.TrimSpace(strings.ToLower(domain))
	org := OrganizationalDomain(domain)
	reported := org
	if reported == "" {
		reported = domain
	}
	status := Status{Result: "neutral", Header: []Prop{{"from", domain}}}
	finish := func(result *DMARCResult) *DMARCResult {
		result.Status = status
		result.Info = status.Format("dmarc")
		return result
	}
	record, err := lookupDMARC(ctx, resolver, domain)
	if err != nil {
		status.Result = "temperror"
		return finish(&DMARCResult{Domain: reported})
	}
	if record == nil {
		status.Result = "none"
		return finish(&DMARCResult{Domain: reported})
	}
	recordDomain := domain
	if record.isOrg {
		recordDomain = org
	}
	status.Header = append(status.Header, Prop{"d", recordDomain})
	var comment []string
	if p := record.tags["p"]; p != "" {
		comment = append(comment, "p="+strings.ToUpper(p))
	}
	if sp := record.tags["sp"]; sp != "" {
		comment = append(comment, "sp="+strings.ToUpper(sp))
	}
	if arc != "" {
		comment = append(comment, "arc="+arc)
	}
	status.Comment = strings.Join(comment, " ")
	policy := record.tags["p"]
	if record.isOrg && record.tags["sp"] != "" {
		policy = record.tags["sp"]
	}
	strictDKIM := record.tags["adkim"] == "s"
	dkimMatch, dkimAligned := alignment(domain, dkimDomains, strictDKIM)
	_, spfAligned := alignment(domain, spfDomains, record.tags["aspf"] == "s")
	underSized := dkimMatch.underSized
	if strictDKIM && !dkimAligned {
		relaxed, _ := alignment(domain, dkimDomains, false)
		underSized = relaxed.underSized
	}
	if dkimAligned || spfAligned {
		status.Result = "pass"
	} else {
		status.Result = "fail"
	}
	result := finish(&DMARCResult{Domain: reported, Policy: policy, DKIMUnderSized: underSized})
	if result.Policy == "quarantine" && record.pct > 0 && record.pct < 100 {
		result.Policy = "quarantine-partial"
	}
	return result
}

// checkBIMI applies mailauth's BIMI gates and looks up the BIMI record; it
// does not fetch or validate the logo.
func checkBIMI(ctx context.Context, resolver Resolver, dmarc *DMARCResult, raw []byte) string {
	status := Status{}
	switch {
	case dmarc.Status.Result != "pass":
		status.Result = "skipped"
		status.Comment = "message failed DMARC"
		if dmarc.Status.Result == "none" {
			status.Comment = "DMARC not enabled"
		}
		return status.Format("bimi")
	case dmarc.Policy == "none" || dmarc.Policy == "quarantine-partial":
		status.Result, status.Comment = "skipped", "too lax DMARC policy"
		return status.Format("bimi")
	case dmarc.DKIMUnderSized > 0:
		status.Result, status.Comment = "skipped", "undersized DKIM signature"
		return status.Format("bimi")
	}
	author := ""
	for _, prop := range dmarc.Status.Header {
		if prop.Name == "from" {
			author = prop.Value
		}
	}
	selector := "default"
	rows, _ := splitMessage(raw)
	for _, row := range rows {
		if row.key == "bimi-selector" {
			fields := tags(row.line[strings.Index(row.line, ":")+1:])
			if strings.ToUpper(fields["v"]) != "BIMI1" {
				status.Result, status.Comment = "fail", "missing bimi version in selector header"
				return status.Format("bimi")
			}
			if fields["s"] != "" {
				selector = fields["s"]
			}
		}
	}
	lookup := func(domain string) (string, bool, error) {
		records, err := resolver.LookupTXT(ctx, selector+"._bimi."+domain)
		if err != nil {
			if IsNotFound(err) {
				return "", false, nil
			}
			return "", false, err
		}
		for _, record := range records {
			if strings.HasPrefix(strings.ToLower(strings.TrimSpace(record)), "v=bimi1") {
				return record, true, nil
			}
		}
		if len(records) > 0 {
			return records[0], true, nil
		}
		return "", false, nil
	}
	record, found, err := lookup(author)
	domain := author
	if err == nil && !found && dmarc.Domain != "" && dmarc.Domain != author {
		record, found, err = lookup(dmarc.Domain)
		domain = dmarc.Domain
	}
	if err != nil {
		status.Result, status.Comment = "temperr", "failed to resolve "+selector+"._bimi."+domain
		return status.Format("bimi")
	}
	if !found {
		status.Result = "none"
		return status.Format("bimi")
	}
	fields := tags(record)
	if strings.ToUpper(fields["v"]) != "BIMI1" {
		status.Result, status.Comment = "fail", "missing bimi version in dns record"
		return status.Format("bimi")
	}
	if fields["l"] == "" && fields["a"] == "" {
		status.Result, status.Comment = "fail", "missing location value in dns record"
		return status.Format("bimi")
	}
	status.Result = "pass"
	status.Header = []Prop{{"selector", selector}, {"d", domain}}
	return status.Format("bimi")
}

// ForInbound adapts Authenticate to the inbound MX.
func ForInbound() inbound.Authenticator {
	return func(ctx context.Context, raw []byte, input inbound.AuthInput) (*inbound.AuthOutcome, error) {
		result, err := Authenticate(ctx, raw, Options{IP: input.IP, Helo: input.Helo, Sender: input.Sender, MTA: input.MTA})
		if err != nil {
			return nil, err
		}
		outcome := &inbound.AuthOutcome{Headers: result.Headers, SPF: result.SPF.Status.Result}
		if result.DMARC != nil {
			outcome.DMARC = result.DMARC.Status.Result
		}
		for _, signature := range result.DKIM {
			if signature.Status.Result == "pass" {
				outcome.DKIMPass = true
			}
		}
		return outcome, nil
	}
}
