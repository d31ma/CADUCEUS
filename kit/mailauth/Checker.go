package mailauth

import "context"

type Checker struct {
	resolver Resolver
}

func NewChecker(resolver Resolver) Checker {
	return Checker{resolver: resolver}
}

func (checker Checker) Check(ctx context.Context, input Input) Results {
	lookup := dnsLookup{resolver: checker.resolver}
	message := newRawMessage(input.Raw)
	results := Results{}
	results.Spf, results.SpfDomain = newSpfSession(lookup, input).run(ctx)
	results.Dkim = dkimVerifier{lookup: lookup, receivedAt: input.ReceivedAt}.verifyAll(ctx, message)
	results = dmarcEvaluator{lookup: lookup, walk: dmarcWalk{lookup: lookup}}.evaluate(ctx, message.fields, results)
	return verdict{}.decide(results)
}
