package mailauth

import (
	"context"
	"slices"
	"strings"
)

type dmarcWalk struct {
	lookup dnsLookup
}

func (walk dmarcWalk) records(ctx context.Context, domain string) ([]dmarcRecord, error) {
	targets := walk.targets(domain)
	var records []dmarcRecord
	var err error
	for index := 0; index < len(targets) && err == nil; index++ {
		var found []dmarcRecord
		found, err = walk.visit(ctx, targets[index])
		records = append(records, found...)
	}
	return records, err
}

func (walk dmarcWalk) visit(ctx context.Context, target string) ([]dmarcRecord, error) {
	texts, err := walk.lookup.text(ctx, walk.prefix()+target)
	if err != nil {
		return nil, err
	}
	return dmarcRecord{}.found(target, texts), nil
}

func (walk dmarcWalk) targets(domain string) []string {
	labels := strings.Split(domain, ".")
	targets := []string{domain}
	for count := min(len(labels)-1, walk.shortenedLabels()); count >= 1; count-- {
		targets = append(targets, strings.Join(labels[len(labels)-count:], "."))
	}
	return targets
}

func (dmarcWalk) prefix() string {
	return "_dmarc."
}

func (dmarcWalk) shortenedLabels() int {
	return 7
}

func (walk dmarcWalk) organization(domain string, records []dmarcRecord) string {
	return walk.choose(records).organization(domain)
}

func (walk dmarcWalk) organizationOf(ctx context.Context, domain string) string {
	records, err := walk.records(ctx, domain)
	return walk.organization(domain, walk.usable(records, err))
}

func (dmarcWalk) usable(records []dmarcRecord, err error) []dmarcRecord {
	if err != nil {
		return nil
	}
	return records
}

func (dmarcWalk) choose(records []dmarcRecord) dmarcRecord {
	index := slices.IndexFunc(records, dmarcRecord.decisive)
	switch {
	case index >= 0:
		return records[index]
	case len(records) > 0:
		return records[len(records)-1]
	default:
		return dmarcRecord{}
	}
}
