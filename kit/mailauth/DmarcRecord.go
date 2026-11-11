package mailauth

import (
	"regexp"
	"slices"
	"strings"
)

type dmarcRecord struct {
	domain            string
	policy            string
	subdomainPolicy   string
	nonexistentPolicy string
	hasNonexistent    bool
	dkimMode          string
	spfMode           string
	publicSuffix      string
}

func (record dmarcRecord) found(domain string, texts []string) []dmarcRecord {
	candidates := slices.DeleteFunc(slices.Clone(texts), record.notDmarc)
	text, err := record.single(candidates)
	if err != nil {
		return nil
	}
	return record.parse(domain, text)
}

func (dmarcRecord) notDmarc(text string) bool {
	return !regexp.MustCompile(`^v[ \t]*=[ \t]*DMARC1[ \t]*(;|$)`).MatchString(text)
}

func (dmarcRecord) single(candidates []string) (string, error) {
	if len(candidates) != 1 {
		return "", failure{result: Outcomes{}.None()}
	}
	return candidates[0], nil
}

func (record dmarcRecord) parse(domain string, text string) []dmarcRecord {
	tags, err := newTagList(text)
	if err != nil {
		return nil
	}
	return []dmarcRecord{record.build(domain, tags)}
}

func (record dmarcRecord) build(domain string, tags tagList) dmarcRecord {
	policy := record.policyOr(tags.compact("p"), Outcomes{}.None())
	subdomainPolicy := record.policyOr(tags.compact("sp"), policy)
	return dmarcRecord{
		domain:            domain,
		policy:            policy,
		subdomainPolicy:   subdomainPolicy,
		nonexistentPolicy: record.policyOr(tags.compact("np"), subdomainPolicy),
		hasNonexistent:    tags.has("np"),
		dkimMode:          strings.ToLower(tags.compact("adkim")),
		spfMode:           strings.ToLower(tags.compact("aspf")),
		publicSuffix:      strings.ToLower(tags.compact("psd")),
	}
}

func (record dmarcRecord) policyOr(text string, fallback string) string {
	policy := strings.ToLower(text)
	if !slices.Contains(record.policies(), policy) {
		return fallback
	}
	return policy
}

func (dmarcRecord) policies() []string {
	outcomes := Outcomes{}
	return []string{outcomes.None(), outcomes.Quarantine(), outcomes.Reject()}
}

func (record dmarcRecord) decisive() bool {
	return record.publicSuffix == record.no() || record.publicSuffix == record.yes()
}

func (dmarcRecord) yes() string { return "y" }
func (dmarcRecord) no() string  { return "n" }

func (record dmarcRecord) organization(start string) string {
	switch {
	case record.domain == "":
		return start
	case record.publicSuffix == record.yes() && record.domain != start:
		return record.child(start)
	default:
		return record.domain
	}
}

func (record dmarcRecord) child(start string) string {
	labels := strings.Split(start, ".")
	count := min(len(strings.Split(record.domain, "."))+1, len(labels))
	return strings.Join(labels[len(labels)-count:], ".")
}

func (record dmarcRecord) applied(from string, nonexistent bool) string {
	switch {
	case record.domain == from:
		return record.policy
	case nonexistent:
		return record.nonexistentPolicy
	default:
		return record.subdomainPolicy
	}
}
