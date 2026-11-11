package mailauth

import (
	"context"
	"errors"
	"slices"
)

type dmarcEvaluator struct {
	lookup dnsLookup
	walk   dmarcWalk
}

func (evaluator dmarcEvaluator) evaluate(ctx context.Context, fields []messageField, results Results) Results {
	from, err := authorDomain{}.find(fields)
	result, policy, err := evaluator.assess(ctx, from, results, err)
	results.FromDomain = from
	results.Dmarc = failure{}.outcome(err, result)
	results.DmarcPolicy = policy
	return results
}

func (evaluator dmarcEvaluator) assess(ctx context.Context, from string, results Results, fromError error) (string, string, error) {
	if fromError != nil {
		return "", Outcomes{}.None(), fromError
	}
	return evaluator.discover(ctx, from, results)
}

func (evaluator dmarcEvaluator) discover(ctx context.Context, from string, results Results) (string, string, error) {
	records, err := evaluator.walk.records(ctx, from)
	if err != nil {
		return "", Outcomes{}.None(), err
	}
	return evaluator.judge(ctx, from, records, results)
}

func (evaluator dmarcEvaluator) judge(ctx context.Context, from string, records []dmarcRecord, results Results) (string, string, error) {
	if len(records) == 0 {
		return Outcomes{}.None(), Outcomes{}.None(), nil
	}
	return evaluator.conclude(ctx, from, records, results)
}

func (evaluator dmarcEvaluator) conclude(ctx context.Context, from string, records []dmarcRecord, results Results) (string, string, error) {
	record := records[0]
	identifiers := append(evaluator.spfIdentifiers(results, record.spfMode), evaluator.dkimIdentifiers(results.Dkim, record.dkimMode)...)
	organization := evaluator.walk.organization(from, records)
	passed := evaluator.anyAligned(ctx, from, organization, identifiers)
	policy := record.applied(from, evaluator.nonexistent(ctx, from, record))
	return evaluator.word(passed), policy, nil
}

func (evaluator dmarcEvaluator) spfIdentifiers(results Results, mode string) []identifier {
	if results.Spf != (Outcomes{}).Pass() {
		return nil
	}
	return []identifier{{domain: results.SpfDomain, mode: mode}}
}

func (evaluator dmarcEvaluator) dkimIdentifiers(dkim []DkimResult, mode string) []identifier {
	var identifiers []identifier
	for _, result := range slices.DeleteFunc(slices.Clone(dkim), DkimResult.failed) {
		identifiers = append(identifiers, identifier{domain: result.Domain, mode: mode})
	}
	return identifiers
}

func (evaluator dmarcEvaluator) anyAligned(ctx context.Context, from string, organization string, identifiers []identifier) bool {
	aligned := false
	for index := 0; index < len(identifiers) && !aligned; index++ {
		aligned = evaluator.aligned(ctx, from, organization, identifiers[index])
	}
	return aligned
}

func (evaluator dmarcEvaluator) aligned(ctx context.Context, from string, organization string, item identifier) bool {
	return item.domain == from || (!item.strict() && evaluator.walk.organizationOf(ctx, item.domain) == organization)
}

func (evaluator dmarcEvaluator) nonexistent(ctx context.Context, from string, record dmarcRecord) bool {
	if !record.hasNonexistent || record.domain == from {
		return false
	}
	found, err := evaluator.recordCount(ctx, from)
	return found == 0 && err == nil
}

func (evaluator dmarcEvaluator) recordCount(ctx context.Context, domain string) (int, error) {
	addresses, addressError := evaluator.lookup.addresses(ctx, domain)
	exchanges, exchangeError := evaluator.lookup.exchanges(ctx, domain)
	return len(addresses) + len(exchanges), errors.Join(addressError, exchangeError)
}

func (dmarcEvaluator) word(passed bool) string {
	if passed {
		return Outcomes{}.Pass()
	}
	return Outcomes{}.Fail()
}
