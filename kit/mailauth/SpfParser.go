package mailauth

import (
	"cmp"
	"net"
	"slices"
	"strconv"
	"strings"
)

type spfParser struct {
	names  spfNames
	limits spfLimits
}

func (parser spfParser) record(texts []string) ([]spfTerm, error) {
	records := slices.DeleteFunc(slices.Clone(texts), parser.notSpf)
	err := parser.single(len(records))
	if err != nil {
		return nil, err
	}
	return parser.parse(records[0])
}

func (parser spfParser) notSpf(text string) bool {
	lower := strings.ToLower(text)
	return lower != parser.names.version() && !strings.HasPrefix(lower, parser.names.version()+" ")
}

func (spfParser) single(count int) error {
	switch {
	case count == 0:
		return failure{result: Outcomes{}.None()}
	case count > 1:
		return failure{result: Outcomes{}.PermError()}
	default:
		return nil
	}
}

func (parser spfParser) parse(text string) ([]spfTerm, error) {
	terms, err := parser.terms(strings.Fields(text)[1:])
	if err != nil {
		return nil, err
	}
	directives := slices.DeleteFunc(slices.Clone(terms), spfTerm.notDirective)
	redirects := slices.DeleteFunc(terms, spfTerm.notRedirect)
	err = parser.atMostOne(len(redirects))
	if err != nil {
		return nil, err
	}
	return slices.Concat(directives, redirects, []spfTerm{parser.neutral()}), nil
}

func (parser spfParser) terms(fields []string) ([]spfTerm, error) {
	terms := make([]spfTerm, len(fields))
	var err error
	for index := 0; index < len(fields) && err == nil; index++ {
		terms[index], err = parser.term(fields[index])
	}
	return terms, err
}

func (parser spfParser) term(field string) (spfTerm, error) {
	result, rest := parser.qualifier(field)
	name, argument := parser.split(rest)
	build, err := parser.builder(name, argument)
	if err != nil {
		return spfTerm{}, err
	}
	return build(result, argument)
}

func (parser spfParser) qualifier(field string) (string, string) {
	result, found := parser.qualifiers()[field[:1]]
	if found {
		return result, field[1:]
	}
	return Outcomes{}.Pass(), field
}

func (spfParser) qualifiers() map[string]string {
	outcomes := Outcomes{}
	return map[string]string{"+": outcomes.Pass(), "-": outcomes.Fail(), "~": outcomes.SoftFail(), "?": outcomes.Neutral()}
}

func (spfParser) split(rest string) (string, string) {
	index := strings.IndexAny(rest, ":/=")
	if index < 0 {
		return strings.ToLower(rest), ""
	}
	return strings.ToLower(rest[:index]), rest[index:]
}

func (parser spfParser) builder(name string, argument string) (func(string, string) (spfTerm, error), error) {
	build, found := parser.builders()[name]
	switch {
	case found:
		return build, nil
	case strings.HasPrefix(argument, "="):
		return parser.ignored, nil
	default:
		return nil, failure{result: Outcomes{}.PermError()}
	}
}

func (parser spfParser) builders() map[string]func(string, string) (spfTerm, error) {
	names := parser.names
	return map[string]func(string, string) (spfTerm, error){
		names.all():      parser.all,
		names.include():  parser.include,
		names.exists():   parser.exists,
		names.address():  parser.address,
		names.exchange(): parser.exchange,
		names.ip4():      parser.ip4,
		names.ip6():      parser.ip6,
		names.pointer():  parser.pointer,
		names.redirect(): parser.redirect,
		names.explain():  parser.ignored,
	}
}

func (parser spfParser) all(result string, argument string) (spfTerm, error) {
	if argument != "" {
		return spfTerm{}, failure{result: Outcomes{}.PermError()}
	}
	return spfTerm{name: parser.names.all(), result: result}, nil
}

func (parser spfParser) include(result string, argument string) (spfTerm, error) {
	domain, err := parser.required(argument, ":")
	return spfTerm{name: parser.names.include(), result: result, domain: domain}, err
}

func (parser spfParser) exists(result string, argument string) (spfTerm, error) {
	domain, err := parser.required(argument, ":")
	return spfTerm{name: parser.names.exists(), result: result, domain: domain}, err
}

func (parser spfParser) redirect(result string, argument string) (spfTerm, error) {
	domain, err := parser.required(argument, "=")
	return spfTerm{name: parser.names.redirect(), result: result, domain: domain}, err
}

func (parser spfParser) pointer(result string, argument string) (spfTerm, error) {
	return spfTerm{name: parser.names.pointer(), result: result}, nil
}

func (spfParser) ignored(result string, argument string) (spfTerm, error) {
	return spfTerm{}, nil
}

func (spfParser) required(argument string, separator string) (string, error) {
	domain, found := strings.CutPrefix(argument, separator)
	if !found || domain == "" {
		return "", failure{result: Outcomes{}.PermError()}
	}
	return domain, nil
}

func (parser spfParser) address(result string, argument string) (spfTerm, error) {
	return parser.host(parser.names.address(), result, argument)
}

func (parser spfParser) exchange(result string, argument string) (spfTerm, error) {
	return parser.host(parser.names.exchange(), result, argument)
}

func (parser spfParser) host(name string, result string, argument string) (spfTerm, error) {
	domainPart, cidr := parser.cutCidr(argument)
	domain, err := parser.optionalDomain(domainPart)
	if err != nil {
		return spfTerm{}, err
	}
	prefix4, prefix6, err := parser.cidr(cidr)
	return spfTerm{name: name, result: result, domain: domain, prefix4: prefix4, prefix6: prefix6}, err
}

func (spfParser) cutCidr(argument string) (string, string) {
	index := strings.Index(argument, "/")
	if index < 0 {
		return argument, ""
	}
	return argument[:index], argument[index:]
}

func (spfParser) optionalDomain(domainPart string) (string, error) {
	domain, found := strings.CutPrefix(domainPart, ":")
	if domainPart != "" && (!found || domain == "") {
		return "", failure{result: Outcomes{}.PermError()}
	}
	return domain, nil
}

func (parser spfParser) cidr(text string) (int, int, error) {
	four, six, _ := strings.Cut(text, "//")
	prefix4, err := parser.prefix(strings.TrimPrefix(four, "/"), parser.limits.bits4())
	if err != nil {
		return 0, 0, err
	}
	prefix6, err := parser.prefix(six, parser.limits.bits6())
	return prefix4, prefix6, err
}

func (spfParser) prefix(text string, bits int) (int, error) {
	number, err := strconv.Atoi(cmp.Or(text, strconv.Itoa(bits)))
	if err != nil || number < 0 || number > bits {
		return 0, failure{result: Outcomes{}.PermError()}
	}
	return number, nil
}

func (parser spfParser) ip4(result string, argument string) (spfTerm, error) {
	return parser.literal(parser.names.ip4(), result, argument, parser.limits.bits4())
}

func (parser spfParser) ip6(result string, argument string) (spfTerm, error) {
	return parser.literal(parser.names.ip6(), result, argument, parser.limits.bits6())
}

func (parser spfParser) literal(name string, result string, argument string, bits int) (spfTerm, error) {
	text, _ := strings.CutPrefix(argument, ":")
	_, network, err := net.ParseCIDR(parser.withPrefix(text, bits))
	if err != nil {
		return spfTerm{}, failure{result: Outcomes{}.PermError()}
	}
	return spfTerm{name: name, result: result, network: network}, parser.family(name, text)
}

func (spfParser) withPrefix(text string, bits int) string {
	if strings.Contains(text, "/") {
		return text
	}
	return text + "/" + strconv.Itoa(bits)
}

func (parser spfParser) family(name string, text string) error {
	if strings.Contains(text, ":") != (name == parser.names.ip6()) {
		return failure{result: Outcomes{}.PermError()}
	}
	return nil
}

func (spfParser) atMostOne(count int) error {
	if count > 1 {
		return failure{result: Outcomes{}.PermError()}
	}
	return nil
}

func (parser spfParser) neutral() spfTerm {
	return spfTerm{name: parser.names.all(), result: Outcomes{}.Neutral()}
}
