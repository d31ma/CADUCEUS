package mailauth

import (
	"regexp"
	"strconv"
	"strings"
)

// Prop is one `ptype.property=value` of an Authentication-Results entry.
type Prop struct{ Name, Value string }

// Status is one method result, formatted as mailauth's formatAuthHeaderRow.
type Status struct {
	Result     string
	Comment    string
	UnderSized int
	Policy     []Prop
	SMTP       []Prop
	Body       []Prop
	Header     []Prop
}

var (
	controlRun   = regexp.MustCompile(`[\x00-\x1F]+`)
	spaceRun     = regexp.MustCompile(`\s+`)
	tokenUnsafe  = regexp.MustCompile(`[\s\x00-\x1F\x7F-\x{FFFF}()<>,;:\\"/\[\]?=]`)
	quoteEscapes = regexp.MustCompile(`["\\]`)
	commentEsc   = regexp.MustCompile(`[\\)]`)
)

func clean(value string) string {
	value = controlRun.ReplaceAllString(value, " ")
	return strings.TrimSpace(spaceRun.ReplaceAllString(value, " "))
}

func escapePropValue(value string) string {
	value = clean(value)
	if !tokenUnsafe.MatchString(value) {
		return value
	}
	return `"` + quoteEscapes.ReplaceAllStringFunc(value, func(c string) string { return `\` + c }) + `"`
}

func escapeCommentValue(value string) string {
	return commentEsc.ReplaceAllStringFunc(clean(value), func(c string) string { return `\` + c })
}

// Format answers the `method=result (comment) ptype.prop=value` row.
func (status Status) Format(method string) string {
	result := status.Result
	if result == "" {
		result = "none"
	}
	parts := []string{method + "=" + result}
	if status.UnderSized > 0 {
		parts = append(parts, "("+escapeCommentValue("undersized signature: "+strconv.Itoa(status.UnderSized)+" bytes unsigned")+")")
	}
	if status.Comment != "" {
		parts = append(parts, "("+escapeCommentValue(status.Comment)+")")
	}
	for _, group := range []struct {
		name  string
		props []Prop
	}{{"policy", status.Policy}, {"smtp", status.SMTP}, {"body", status.Body}, {"header", status.Header}} {
		for _, prop := range group.props {
			if prop.Value != "" {
				parts = append(parts, group.name+"."+prop.Name+"="+escapePropValue(prop.Value))
			}
		}
	}
	return strings.Join(parts, " ")
}

var (
	lineBreakPrefix = regexp.MustCompile(`^[^\n\r]*(\r?\n|\r)`)
	lastWord        = regexp.MustCompile(`(\s+)[^\s]*$`)
	nextWord        = regexp.MustCompile(`^[^\s]+(\s*)`)
)

// foldLines ports libmime.foldLines(str, lineLength) (afterSpace false).
func foldLines(text string, lineLength int) string {
	var result strings.Builder
	position, length := 0, len(text)
	for position < length {
		end := position + lineLength
		if end > length {
			end = length
		}
		line := text[position:end]
		if len(line) < lineLength {
			result.WriteString(line)
			break
		}
		if match := lineBreakPrefix.FindString(line); match != "" {
			result.WriteString(match)
			position += len(match)
			continue
		}
		if match := lastWord.FindString(line); match != "" && len(match) < len(line) {
			line = line[:len(line)-len(match)]
		} else if groups := nextWord.FindStringSubmatch(text[position+len(line):]); groups != nil {
			line += groups[0][:len(groups[0])-len(groups[1])]
		}
		result.WriteString(line)
		position += len(line)
		if position < length {
			result.WriteString("\r\n")
		}
	}
	return result.String()
}

// formatDomain lowercases and drops the root label, as mailauth did.
func formatDomain(domain string) string {
	return strings.TrimRight(strings.TrimSpace(strings.ToLower(domain)), ".")
}

// multiLabelSuffixes are public suffixes of more than one label that are
// common in mail. mailauth used the full Public Suffix List (tldts, ICANN
// and private sections); this list is the stdlib stand-in.
var multiLabelSuffixes = map[string]bool{}

func init() {
	for _, suffix := range strings.Fields(`
		co.uk org.uk ac.uk gov.uk ltd.uk plc.uk me.uk net.uk sch.uk nhs.uk police.uk
		com.au net.au org.au edu.au gov.au asn.au id.au
		co.nz net.nz org.nz govt.nz ac.nz school.nz geek.nz
		co.jp ne.jp or.jp ac.jp go.jp ad.jp ed.jp gr.jp lg.jp
		co.kr or.kr ne.kr go.kr ac.kr re.kr
		com.br net.br org.br gov.br edu.br art.br
		com.cn net.cn org.cn gov.cn edu.cn ac.cn
		com.hk net.hk org.hk gov.hk edu.hk idv.hk
		com.tw net.tw org.tw gov.tw edu.tw idv.tw
		com.sg net.sg org.sg gov.sg edu.sg per.sg
		com.my net.my org.my gov.my edu.my
		co.in net.in org.in gov.in ac.in res.in firm.in gen.in ind.in
		co.za net.za org.za gov.za ac.za web.za
		com.mx net.mx org.mx gob.mx edu.mx
		com.ar net.ar org.ar gob.ar edu.ar
		com.tr net.tr org.tr gov.tr edu.tr bel.tr
		com.ua net.ua org.ua gov.ua edu.ua in.ua
		co.il org.il net.il ac.il gov.il muni.il
		com.ru net.ru org.ru msk.ru spb.ru
		com.pl net.pl org.pl gov.pl edu.pl waw.pl
		co.id or.id web.id ac.id go.id my.id
		com.ph net.ph org.ph gov.ph edu.ph
		com.vn net.vn org.vn gov.vn edu.vn
		com.pk net.pk org.pk gov.pk edu.pk
		com.ng org.ng gov.ng edu.ng
		co.ke or.ke ne.ke go.ke ac.ke
		com.eg org.eg gov.eg edu.eg
		com.sa net.sa org.sa gov.sa edu.sa
		co.th in.th or.th go.th ac.th
		com.co net.co org.co gov.co edu.co
		com.pe org.pe gob.pe edu.pe
		com.es org.es nom.es gob.es edu.es
		com.pt org.pt gov.pt edu.pt
		co.at or.at gv.at ac.at
		com.gr org.gr gov.gr edu.gr
		com.ro org.ro
		co.hu org.hu
		com.de
		github.io gitlab.io herokuapp.com appspot.com blogspot.com netlify.app vercel.app
		pages.dev workers.dev web.app firebaseapp.com azurewebsites.net cloudapp.net
		amazonaws.com cloudfront.net elasticbeanstalk.com s3.amazonaws.com fly.dev onrender.com
	`) {
		multiLabelSuffixes[suffix] = true
	}
}

// OrganizationalDomain approximates tldts.getDomain: the registrable domain
// under the public suffix. It answers "" for a bare suffix.
func OrganizationalDomain(domain string) string {
	domain = formatDomain(domain)
	labels := strings.Split(domain, ".")
	if len(labels) < 2 {
		return ""
	}
	// Longest known multi-label suffix wins.
	for size := len(labels) - 1; size >= 2; size-- {
		suffix := strings.Join(labels[len(labels)-size:], ".")
		if multiLabelSuffixes[suffix] {
			if len(labels) == size {
				return ""
			}
			return strings.Join(labels[len(labels)-size-1:], ".")
		}
	}
	if multiLabelSuffixes[domain] {
		return ""
	}
	return strings.Join(labels[len(labels)-2:], ".")
}

type alignedDomain struct {
	domain     string
	underSized int
}

// alignment ports getAlignment: strict needs the identical domain, relaxed a
// shared organizational domain.
func alignment(fromDomain string, candidates []alignedDomain, strict bool) (alignedDomain, bool) {
	ordered := append([]alignedDomain(nil), candidates...)
	for i := 1; i < len(ordered); i++ {
		for j := i; j > 0 && ordered[j].underSized < ordered[j-1].underSized; j-- {
			ordered[j], ordered[j-1] = ordered[j-1], ordered[j]
		}
	}
	if strict {
		from := formatDomain(fromDomain)
		if from == "" {
			return alignedDomain{}, false
		}
		for _, candidate := range ordered {
			if formatDomain(candidate.domain) == from {
				return candidate, true
			}
		}
		return alignedDomain{}, false
	}
	org := func(domain string) string {
		if value := OrganizationalDomain(domain); value != "" {
			return value
		}
		return formatDomain(domain)
	}
	fromOrg := org(fromDomain)
	if fromOrg == "" {
		return alignedDomain{}, false
	}
	for _, candidate := range ordered {
		if org(candidate.domain) == fromOrg {
			return candidate, true
		}
	}
	return alignedDomain{}, false
}
