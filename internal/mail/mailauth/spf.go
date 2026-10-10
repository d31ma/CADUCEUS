package mailauth

import (
	"context"
	"fmt"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	maxResolveCount = 10
	maxVoidCount    = 2
	maxPTRRecords   = 10
)

// SPFResult is one SPF evaluation.
type SPFResult struct {
	Domain       string
	ClientIP     string
	Helo         string
	EnvelopeFrom string
	Status       Status
	// Header is the folded Received-SPF header line.
	Header string
	// Info is the Authentication-Results entry.
	Info string
}

// spfFailure is a thrown SPF result (mailauth's err.spfResult).
type spfFailure struct {
	kind string // none, permerror, temperror
	text string
}

func (failure *spfFailure) Error() string { return failure.text }

type spfMatch struct {
	qualifier string
	failure   *spfFailure
}

type limitedResolver struct {
	resolver   Resolver
	count      int
	voids      int
	countFirst bool
}

var lookupDomain = regexp.MustCompile(`(?i)^([\x20-\x2D\x2f-\x7e]+\.)+[a-z]+[a-z\-0-9]*$`)

func (limited *limitedResolver) begin(domain string) error {
	if limited.countFirst {
		limited.count++
	} else {
		limited.countFirst = true
	}
	if limited.count > maxResolveCount {
		return &spfFailure{kind: "permerror", text: "Too many DNS requests"}
	}
	if !lookupDomain.MatchString(domain) {
		return &spfFailure{kind: "permerror", text: "Invalid domain " + domain}
	}
	return nil
}

// classify turns a lookup error into an SPF failure, or nil for a counted
// void lookup.
func (limited *limitedResolver) classify(domain string, err error) error {
	switch {
	case IsNotFound(err):
		limited.voids++
		if limited.voids > maxVoidCount {
			return &spfFailure{kind: "permerror", text: "Too many void DNS results"}
		}
		return nil
	case IsTimeout(err):
		return &spfFailure{kind: "temperror", text: "DNS timeout"}
	case isRefused(err):
		return &spfFailure{kind: "temperror", text: "DNS request refused by server when resolving " + domain}
	}
	return err
}

func (limited *limitedResolver) txt(ctx context.Context, domain string) ([]string, error) {
	if err := limited.begin(domain); err != nil {
		return nil, err
	}
	records, err := limited.resolver.LookupTXT(ctx, domain)
	if err != nil {
		return nil, limited.classify(domain, err)
	}
	return records, nil
}

func (limited *limitedResolver) mx(ctx context.Context, domain string) ([]string, error) {
	if err := limited.begin(domain); err != nil {
		return nil, err
	}
	records, err := limited.resolver.LookupMX(ctx, domain)
	if err != nil {
		return nil, limited.classify(domain, err)
	}
	var hosts []string
	for _, record := range records {
		hosts = append(hosts, strings.TrimSuffix(record.Host, "."))
	}
	return hosts, nil
}

func (limited *limitedResolver) ptr(ctx context.Context, address netip.Addr) ([]string, error) {
	name := ptrHostname(address)
	if err := limited.begin(name); err != nil {
		return nil, err
	}
	names, err := limited.resolver.LookupAddr(ctx, address.String())
	if err != nil {
		return nil, limited.classify(name, err)
	}
	for index := range names {
		names[index] = strings.TrimSuffix(names[index], ".")
	}
	return names, nil
}

// addresses answers the client's family of A/AAAA records; a lookup is void
// only when both families are empty (mailauth's dual-stack resolver).
func (limited *limitedResolver) addresses(ctx context.Context, host string, ipv6 bool) ([]netip.Addr, error) {
	if err := limited.begin(host); err != nil {
		return nil, err
	}
	v4, err4 := limited.resolver.LookupIP(ctx, "ip4", host)
	v6, err6 := limited.resolver.LookupIP(ctx, "ip6", host)
	void4, void6 := err4 != nil && IsNotFound(err4), err6 != nil && IsNotFound(err6)
	if ipv6 && err6 != nil && !void6 {
		return nil, limited.classify(host, err6)
	}
	if !ipv6 && err4 != nil && !void4 {
		return nil, limited.classify(host, err4)
	}
	if void4 && void6 {
		if ipv6 {
			return nil, limited.classify(host, err6)
		}
		return nil, limited.classify(host, err4)
	}
	if ipv6 {
		return v6, nil
	}
	return v4, nil
}

func ptrHostname(address netip.Addr) string {
	bytes := address.AsSlice()
	var parts []string
	if len(bytes) == 4 {
		for index := len(bytes) - 1; index >= 0; index-- {
			parts = append(parts, strconv.Itoa(int(bytes[index])))
		}
		return strings.Join(parts, ".") + ".in-addr.arpa"
	}
	hexDigits := fmt.Sprintf("%x", bytes)
	for index := len(hexDigits) - 1; index >= 0; index-- {
		parts = append(parts, hexDigits[index:index+1])
	}
	return strings.Join(parts, ".") + ".ip6.arpa"
}

type spfContext struct {
	sender, helo, mta string
	ip                netip.Addr
	ipText            string
	resolver          *limitedResolver
	base              Resolver
	resolvedPTR       []string
	now               func() time.Time
}

var macroPattern = regexp.MustCompile(`(?i)%%|%_|%-|%\{([^}]+)\}|%`)

// expand ports mailauth's SPF macro expansion.
func (spf *spfContext) expand(input string) (output string, err error) {
	senderLocal, senderDomain := "", spf.sender
	if at := strings.Index(spf.sender, "@"); at >= 0 {
		senderLocal, senderDomain = spf.sender[:at], spf.sender[at+1:]
	}
	output = macroPattern.ReplaceAllStringFunc(input, func(match string) string {
		if err != nil {
			return ""
		}
		switch match {
		case "%":
			err = &spfFailure{kind: "permerror", text: "Unexpected % in macro"}
			return ""
		case "%%":
			return "%"
		case "%_":
			return " "
		case "%-":
			return "%20"
		}
		body := match[2 : len(match)-1]
		letter, rest := strings.ToLower(body[:1]), body[1:]
		var value string
		switch letter {
		case "s":
			value = spf.sender
		case "l":
			value = senderLocal
		case "o", "d", "p":
			value = senderDomain
		case "i":
			if spf.ip.Is4() {
				value = spf.ip.String()
			} else {
				hexDigits := fmt.Sprintf("%x", spf.ip.AsSlice())
				value = strings.Join(strings.Split(hexDigits, ""), ".")
			}
		case "v":
			value = "ip6"
			if spf.ip.Is4() {
				value = "in-addr"
			}
		case "h":
			value = spf.helo
			if value == "" {
				value = "[" + spf.ip.String() + "]"
			}
		case "c":
			value = spf.ip.String()
		case "r":
			value = spf.mta
		case "t":
			value = strconv.FormatInt(spf.now().Unix(), 10)
		default:
			err = &spfFailure{kind: "permerror", text: fmt.Sprintf("Unknown macro letter %q", body[:1])}
			return ""
		}
		digits := 0
		index := 0
		for index < len(rest) && rest[index] >= '0' && rest[index] <= '9' {
			digits = digits*10 + int(rest[index]-'0')
			index++
		}
		reversed := false
		if index < len(rest) && rest[index] == 'r' {
			reversed = true
			index++
		}
		delimiters := ""
		for _, character := range rest[index:] {
			if strings.ContainsRune(".-+,/_=", character) && !strings.ContainsRune(delimiters, character) {
				delimiters += string(character)
			}
		}
		if delimiters == "" {
			delimiters = "."
		}
		if !reversed && digits == 0 && delimiters == "." {
			return value
		}
		parts := strings.FieldsFunc(value, func(r rune) bool { return strings.ContainsRune(delimiters, r) })
		if strings.HasPrefix(value, ".") || strings.HasSuffix(value, ".") || strings.Contains(value, "..") {
			parts = strings.Split(value, ".")
		}
		if reversed {
			for left, right := 0, len(parts)-1; left < right; left, right = left+1, right-1 {
				parts[left], parts[right] = parts[right], parts[left]
			}
		}
		if digits > 0 && digits < len(parts) {
			parts = parts[len(parts)-digits:]
		}
		return strings.Join(parts, ".")
	})
	return output, err
}

var cidrPattern = regexp.MustCompile(`^(.*?)(/\d+)?(//\d+)?$`)

type cidrValue struct {
	domain       string
	cidr4, cidr6 int // -1 when absent
}

func parseCIDRValue(value, fallback, mechanism string) (cidrValue, error) {
	result := cidrValue{cidr4: -1, cidr6: -1}
	if value != "" {
		match := cidrPattern.FindStringSubmatch(value)
		if match == nil || regexp.MustCompile(`^/0+[1-9]`).MatchString(match[2]) || regexp.MustCompile(`^//0+[1-9]`).MatchString(match[3]) {
			return result, &spfFailure{kind: "permerror", text: "invalid address definition: " + value}
		}
		result.domain = match[1]
		if match[2] != "" {
			result.cidr4, _ = strconv.Atoi(match[2][1:])
		}
		if match[3] != "" {
			result.cidr6, _ = strconv.Atoi(match[3][2:])
		}
		if mechanism == "ip6" && result.cidr4 > 0 && result.cidr6 < 0 {
			result.cidr6, result.cidr4 = result.cidr4, -1
		}
	}
	result.domain = strings.TrimSpace(strings.ToLower(result.domain))
	if result.domain == "" {
		result.domain = fallback
	}
	parsed, err := netip.ParseAddr(result.domain)
	isIPv6 := err == nil && parsed.Is6()
	if (result.cidr4 > 32 && !isIPv6) || result.cidr6 > 128 {
		return result, &spfFailure{kind: "permerror", text: "invalid cidr definition: " + value}
	}
	return result, nil
}

func matches(address, candidate netip.Addr, bits int) bool {
	address, candidate = address.Unmap(), candidate.Unmap()
	if address.Is4() != candidate.Is4() {
		return false
	}
	if bits < 0 {
		return address == candidate
	}
	prefix, err := candidate.Prefix(bits)
	return err == nil && prefix.Contains(address)
}

var (
	modifierPattern  = regexp.MustCompile(`^[^:/]+=`)
	redirectPattern  = regexp.MustCompile(`^([\x21-\x2D\x2f-\x7e]+\.)+[a-z]+[a-z\-0-9]*$`)
	qualifierPattern = regexp.MustCompile(`^[?\-~+]`)
	allPattern       = regexp.MustCompile(`(?i)^[?\-~+]?all$`)
	mappedIPv4       = regexp.MustCompile(`(?i)^[:A-F]+:((\d+\.){3}\d+)$`)
)

type spfTerm struct {
	text     string
	modifier string
	value    string
}

// verifyDomain ports spfVerify. It answers nil (no decision), a match, or a
// thrown failure.
func (spf *spfContext) verifyDomain(ctx context.Context, domain string) (*spfMatch, error) {
	responses, err := spf.resolver.txt(ctx, domain)
	if err != nil {
		return nil, err
	}
	var record []spfTerm
	found := false
	for _, row := range responses {
		parts := strings.Fields(strings.TrimSpace(row))
		if len(parts) == 0 || strings.ToLower(parts[0]) != "v=spf1" {
			continue
		}
		if found {
			return nil, &spfFailure{kind: "permerror", text: "multiple SPF records found for " + domain}
		}
		found = true
		for _, character := range row {
			if character < 0x20 || character > 0x7e {
				return nil, &spfFailure{kind: "permerror", text: "DNS response includes invalid characters"}
			}
		}
		for _, part := range parts[1:] {
			record = append(record, spfTerm{text: part})
		}
	}
	if !found {
		return nil, &spfFailure{kind: "none", text: "no SPF records found for " + domain}
	}
	match, err := spf.evaluate(ctx, domain, record)
	if err != nil {
		return nil, err
	}
	if match == nil {
		return &spfMatch{qualifier: "?"}, nil
	}
	return match, nil
}

func (spf *spfContext) evaluate(ctx context.Context, domain string, record []spfTerm) (*spfMatch, error) {
	// Modifiers move to the end, in reverse order of appearance, as the
	// JavaScript splice/push loop left them.
	var mechanisms, modifiers []spfTerm
	for index := len(record) - 1; index >= 0; index-- {
		part := record[index].text
		if modifierPattern.MatchString(part) {
			split := strings.Index(part, "=")
			name := strings.ToLower(part[:split])
			value, err := spf.expand(part[split+1:])
			if err != nil {
				return nil, err
			}
			value = strings.TrimSuffix(value, ".")
			if value == "" {
				return nil, &spfFailure{kind: "permerror", text: "Empty modifier value for " + name}
			}
			if name == "redirect" && !redirectPattern.MatchString(value) {
				return nil, &spfFailure{kind: "permerror", text: "Invalid redirect target " + value}
			}
			modifiers = append(modifiers, spfTerm{modifier: name, value: value})
			continue
		}
		mechanism := strings.ToLower(regexp.MustCompile(`[:/=]`).Split(part, 2)[0])
		mechanism = qualifierPattern.ReplaceAllString(mechanism, "")
		switch mechanism {
		case "all", "include", "a", "mx", "ip4", "ip6", "exists", "ptr":
		default:
			return nil, &spfFailure{kind: "permerror", text: "Unknown mechanism " + mechanism}
		}
		mechanisms = append([]spfTerm{record[index]}, mechanisms...)
	}
	redirects := 0
	for _, modifier := range modifiers {
		if modifier.modifier == "redirect" {
			redirects++
		}
	}
	if redirects > 1 {
		return nil, &spfFailure{kind: "permerror", text: "more than 1 redirect found"}
	}
	hasAll := false
	for _, term := range mechanisms {
		if allPattern.MatchString(term.text) {
			hasAll = true
		}
	}
	for _, term := range mechanisms {
		match, err := spf.mechanism(ctx, domain, term.text)
		if err != nil || match != nil {
			return match, err
		}
	}
	for _, modifier := range modifiers {
		if modifier.modifier != "redirect" || hasAll {
			continue
		}
		sub, err := spf.verifyDomain(ctx, modifier.value)
		if err != nil {
			if failure, ok := err.(*spfFailure); ok {
				if failure.kind == "none" {
					failure.kind = "permerror"
				}
				return nil, failure
			}
			// Other errors fall through, as the JavaScript ignored them.
			continue
		}
		if sub != nil {
			return sub, nil
		}
	}
	return nil, nil
}

func (spf *spfContext) mechanism(ctx context.Context, domain, part string) (*spfMatch, error) {
	key, value, qualifier := "", "", "+"
	if colon := strings.Index(part, ":"); colon == len(part)-1 {
		return nil, &spfFailure{kind: "permerror", text: "unexpected empty value"}
	} else if colon >= 0 {
		key, value = part[:colon], part[colon+1:]
	} else if slash := strings.Index(part, "/"); slash >= 0 {
		key, value = part[:slash], part[slash:]
	} else {
		key = part
	}
	if qualifierPattern.MatchString(key) {
		qualifier, key = key[:1], key[1:]
	}
	ipv6 := spf.ip.Is6()
	switch strings.ToLower(key) {
	case "all":
		if value != "" {
			return nil, &spfFailure{kind: "permerror", text: "unexpected extension for all"}
		}
		return &spfMatch{qualifier: qualifier}, nil
	case "include":
		target, err := spf.expand(value)
		if err != nil {
			return nil, err
		}
		sub, err := spf.verifyDomain(ctx, strings.TrimSuffix(target, "."))
		if err != nil {
			if failure, ok := err.(*spfFailure); ok {
				if failure.kind == "none" {
					failure.kind = "permerror"
				}
				return &spfMatch{failure: failure}, nil
			}
			return nil, nil
		}
		if sub != nil && sub.qualifier == "+" {
			return &spfMatch{qualifier: qualifier}, nil
		}
		if sub != nil && sub.failure != nil {
			return sub, nil
		}
	case "ip4", "ip6":
		mechanism := strings.ToLower(key)
		parsed, err := parseCIDRValue(value, "", mechanism)
		if err != nil {
			return nil, err
		}
		if parsed.domain == "" {
			return nil, &spfFailure{kind: "permerror", text: "bare IP address"}
		}
		original, rangeText := parsed.domain, parsed.domain
		if mapped := mappedIPv4.FindStringSubmatch(rangeText); mapped != nil {
			rangeText = mapped[1]
		}
		candidate, err := netip.ParseAddr(rangeText)
		if err != nil {
			return nil, &spfFailure{kind: "permerror", text: "invalid IP address"}
		}
		originalAddr, originalErr := netip.ParseAddr(original)
		if mechanism == "ip6" && ipv6 && originalErr == nil && originalAddr.Is6() && candidate.Is4() && parsed.cidr4 == 0 {
			return &spfMatch{qualifier: qualifier}, nil
		}
		if parsed.cidr6 >= 0 && candidate.Is4() {
			return nil, &spfFailure{kind: "permerror", text: "invalid CIDR for IP"}
		}
		family := 4
		if candidate.Is6() {
			family = 6
		}
		clientFamily := 4
		if ipv6 {
			clientFamily = 6
		}
		if family != clientFamily || strconv.Itoa(family) != mechanism[2:] {
			return nil, nil
		}
		bits := parsed.cidr4
		if candidate.Is6() {
			bits = parsed.cidr6
		}
		if matches(spf.ip, candidate, bits) {
			return &spfMatch{qualifier: qualifier}, nil
		}
	case "a":
		parsed, err := parseCIDRValue(value, domain, "a")
		if err != nil {
			return nil, err
		}
		target, err := spf.expand(parsed.domain)
		if err != nil {
			return nil, err
		}
		records, err := spf.resolver.addresses(ctx, target, ipv6)
		if err != nil {
			return nil, err
		}
		bits := parsed.cidr4
		if ipv6 {
			bits = parsed.cidr6
		}
		for _, record := range records {
			if matches(spf.ip, record, bits) {
				return &spfMatch{qualifier: qualifier}, nil
			}
		}
	case "mx":
		parsed, err := parseCIDRValue(value, domain, "mx")
		if err != nil {
			return nil, err
		}
		target, err := spf.expand(parsed.domain)
		if err != nil {
			return nil, err
		}
		hosts, err := spf.resolver.mx(ctx, target)
		if err != nil {
			return nil, err
		}
		sub := &limitedResolver{resolver: spf.base, countFirst: true}
		bits := parsed.cidr4
		if ipv6 {
			bits = parsed.cidr6
		}
		for _, host := range hosts {
			if host == "" {
				continue
			}
			records, err := sub.addresses(ctx, host, ipv6)
			if err != nil {
				return nil, err
			}
			for _, record := range records {
				if matches(spf.ip, record, bits) {
					return &spfMatch{qualifier: qualifier}, nil
				}
			}
		}
	case "exists":
		target, err := spf.expand(value)
		if err != nil {
			return nil, err
		}
		if err := spf.resolver.begin(target); err != nil {
			return nil, err
		}
		records, err := spf.base.LookupIP(ctx, "ip4", target)
		if err != nil {
			if err := spf.resolver.classify(target, err); err != nil {
				return nil, err
			}
		}
		if len(records) > 0 {
			return &spfMatch{qualifier: qualifier}, nil
		}
	case "ptr":
		parsed, err := parseCIDRValue(value, "", "ptr")
		if err != nil {
			return nil, err
		}
		if parsed.cidr4 >= 0 || parsed.cidr6 >= 0 {
			return nil, &spfFailure{kind: "permerror", text: "invalid domain-spec definition: " + value}
		}
		specification := value
		if specification == "" {
			specification = "%{d}"
		}
		target, err := spf.expand(specification)
		if err != nil {
			return nil, err
		}
		target = formatDomain(target)
		if spf.resolvedPTR == nil {
			names, err := spf.resolver.ptr(ctx, spf.ip)
			if err != nil {
				return nil, err
			}
			spf.resolvedPTR = append([]string{}, names...)
		}
		sub := &limitedResolver{resolver: spf.base, countFirst: true}
		for index, name := range spf.resolvedPTR {
			if index >= maxPTRRecords {
				break
			}
			records, err := sub.addresses(ctx, name, ipv6)
			if err != nil {
				continue
			}
			for _, record := range records {
				if record.Unmap() == spf.ip.Unmap() {
					formatted := formatDomain(name)
					if formatted == target || strings.HasSuffix(formatted, "."+target) {
						return &spfMatch{qualifier: qualifier}, nil
					}
				}
			}
		}
	}
	return nil, nil
}

var validSPFDomain = regexp.MustCompile(`^(?i)([a-z0-9]([a-z0-9-]*[a-z0-9])?\.)+[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

// checkSPF ports mailauth's spf(): the result, its comment, and the headers.
func checkSPF(ctx context.Context, resolver Resolver, sender, ipText, helo, mta string, now func() time.Time) SPFResult {
	if sender == "" {
		sender = "postmaster@" + helo
	}
	if mapped := regexp.MustCompile(`(?i)^[:A-F]+:((\d+\.){3}\d+)$`).FindStringSubmatch(ipText); mapped != nil {
		ipText = mapped[1]
	}
	if at := strings.Index(sender, "@"); at < 0 {
		sender = "postmaster@" + sender
	} else if at == 0 {
		sender = "postmaster" + sender
	}
	parts := strings.Split(sender, "@")
	domain := strings.TrimSpace(strings.ToLower(parts[len(parts)-1]))
	if domain == "" {
		domain = "-"
	}
	limited := &limitedResolver{resolver: resolver}
	spf := &spfContext{sender: sender, helo: helo, mta: mta, ipText: ipText, resolver: limited, base: resolver, now: now}

	var match *spfMatch
	var failure *spfFailure
	address, addressErr := netip.ParseAddr(ipText)
	switch {
	case !validSPFDomain.MatchString(domain) || len(domain) > 253:
		failure = &spfFailure{kind: "none", text: "Invalid domain " + domain}
	case addressErr != nil:
		// spfVerify answered false for a missing or invalid IP.
	default:
		spf.ip = address.Unmap()
		result, err := spf.verifyDomain(ctx, domain)
		if err != nil {
			if thrown, ok := err.(*spfFailure); ok {
				failure = thrown
			} else {
				failure = &spfFailure{kind: "temperror", text: err.Error()}
			}
		} else {
			match = result
		}
	}
	if match != nil && match.failure != nil {
		failure, match = match.failure, nil
	}

	kind := "?"
	text := ""
	if failure != nil {
		kind, text = failure.kind, failure.text
	} else if match != nil {
		kind = match.qualifier
	}
	status := Status{SMTP: []Prop{{"mailfrom", sender}, {"helo", helo}}}
	suffix := ""
	if text != "" {
		suffix = ": " + text
	}
	switch kind {
	case "+":
		status.Result, status.Comment = "pass", fmt.Sprintf("%s: domain of %s designates %s as permitted sender", mta, sender, ipText)
	case "~":
		status.Result, status.Comment = "softfail", fmt.Sprintf("%s: domain of transitioning %s does not designate %s as permitted sender", mta, sender, ipText)
	case "-":
		status.Result, status.Comment = "fail", fmt.Sprintf("%s: domain of %s does not designate %s as permitted sender", mta, sender, ipText)
	case "?":
		status.Result, status.Comment = "neutral", fmt.Sprintf("%s: %s is neither permitted nor denied by domain of %s", mta, ipText, sender)
	case "none":
		status.Result, status.Comment = "none", fmt.Sprintf("%s: %s does not designate permitted sender hosts", mta, domain)
	case "permerror":
		status.Result, status.Comment = "permerror", fmt.Sprintf("%s: permanent error in processing during lookup of %s%s", mta, sender, suffix)
	default:
		status.Result, status.Comment = "temperror", fmt.Sprintf("%s: error in processing during lookup of %s%s", mta, sender, suffix)
	}
	result := SPFResult{Domain: domain, ClientIP: ipText, Helo: helo, EnvelopeFrom: sender, Status: status}
	header := "Received-SPF: " + status.Result
	if status.Comment != "" {
		header += " (" + escapeCommentValue(status.Comment) + ")"
	}
	header += " client-ip=" + ipText + ";"
	result.Header = foldLines(header, 160)
	result.Info = status.Format("spf")
	return result
}
