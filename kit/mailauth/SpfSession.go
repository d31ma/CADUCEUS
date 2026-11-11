package mailauth

import (
	"cmp"
	"context"
	"net"
)

type spfSession struct {
	lookup   dnsLookup
	identity spfIdentity
	remote   net.IP
	lookups  int
	voids    int
	limits   spfLimits
}

func newSpfSession(lookup dnsLookup, input Input) *spfSession {
	return &spfSession{lookup: lookup, identity: newSpfIdentity(input), remote: input.RemoteIp}
}

func (session *spfSession) run(ctx context.Context) (string, string) {
	return session.start(ctx), session.identity.domain
}

func (session *spfSession) start(ctx context.Context) string {
	if session.identity.domain == "" || session.remote == nil {
		return Outcomes{}.None()
	}
	return session.checkHost(ctx, session.identity.domain)
}

func (session *spfSession) checkHost(ctx context.Context, domain string) string {
	result, err := session.evaluate(ctx, domain)
	return failure{}.outcome(err, result)
}

func (session *spfSession) evaluate(ctx context.Context, domain string) (string, error) {
	texts, err := session.lookup.text(ctx, domain)
	if err != nil {
		return "", err
	}
	terms, err := spfParser{}.record(texts)
	if err != nil {
		return "", err
	}
	return session.firstResult(ctx, domain, terms)
}

func (session *spfSession) firstResult(ctx context.Context, domain string, terms []spfTerm) (string, error) {
	result := ""
	var err error
	for index := 0; index < len(terms) && result == "" && err == nil; index++ {
		result, err = session.apply(ctx, domain, terms[index])
	}
	return result, err
}

func (session *spfSession) apply(ctx context.Context, domain string, term spfTerm) (string, error) {
	return session.mechanisms()[term.name](ctx, domain, term)
}

func (session *spfSession) mechanisms() map[string]func(context.Context, string, spfTerm) (string, error) {
	names := spfNames{}
	return map[string]func(context.Context, string, spfTerm) (string, error){
		names.all():      session.all,
		names.include():  session.include,
		names.address():  session.address,
		names.exchange(): session.exchange,
		names.ip4():      session.literal,
		names.ip6():      session.literal,
		names.exists():   session.exists,
		names.pointer():  session.pointer,
		names.redirect(): session.redirect,
	}
}

func (session *spfSession) all(ctx context.Context, domain string, term spfTerm) (string, error) {
	return term.result, nil
}

func (session *spfSession) literal(ctx context.Context, domain string, term spfTerm) (string, error) {
	return term.outcome(term.network.Contains(session.remote)), nil
}

func (session *spfSession) pointer(ctx context.Context, domain string, term spfTerm) (string, error) {
	return "", session.countLookup()
}

func (session *spfSession) include(ctx context.Context, domain string, term spfTerm) (string, error) {
	target, err := session.target(domain, term)
	if err != nil {
		return "", err
	}
	matched, err := session.included(session.checkHost(ctx, target))
	return term.outcome(matched), err
}

func (session *spfSession) included(result string) (bool, error) {
	outcomes := Outcomes{}
	refusals := map[string]error{
		outcomes.TempError(): failure{result: outcomes.TempError()},
		outcomes.PermError(): failure{result: outcomes.PermError()},
		outcomes.None():      failure{result: outcomes.PermError()},
	}
	return result == outcomes.Pass(), refusals[result]
}

func (session *spfSession) redirect(ctx context.Context, domain string, term spfTerm) (string, error) {
	target, err := session.target(domain, term)
	if err != nil {
		return "", err
	}
	return session.redirected(session.checkHost(ctx, target))
}

func (session *spfSession) redirected(result string) (string, error) {
	if result == (Outcomes{}).None() {
		return "", failure{result: Outcomes{}.PermError()}
	}
	return result, nil
}

func (session *spfSession) address(ctx context.Context, domain string, term spfTerm) (string, error) {
	target, err := session.target(domain, term)
	if err != nil {
		return "", err
	}
	addresses, err := session.resolve(ctx, target)
	if err != nil {
		return "", err
	}
	return term.outcome(session.network(term).coversAny(addresses)), nil
}

func (session *spfSession) exists(ctx context.Context, domain string, term spfTerm) (string, error) {
	target, err := session.target(domain, term)
	if err != nil {
		return "", err
	}
	addresses, err := session.resolve(ctx, target)
	return term.outcome(len(addresses) > 0), err
}

func (session *spfSession) exchange(ctx context.Context, domain string, term spfTerm) (string, error) {
	target, err := session.target(domain, term)
	if err != nil {
		return "", err
	}
	exchanges, err := session.exchanges(ctx, target)
	if err != nil {
		return "", err
	}
	addresses, err := session.exchangeAddresses(ctx, exchanges)
	return term.outcome(session.network(term).coversAny(addresses)), err
}

func (session *spfSession) exchanges(ctx context.Context, target string) ([]*net.MX, error) {
	exchanges, err := session.lookup.exchanges(ctx, target)
	if err != nil {
		return nil, err
	}
	err = session.countVoid(len(exchanges))
	if err != nil {
		return nil, err
	}
	return exchanges, session.limitExchanges(len(exchanges))
}

func (session *spfSession) exchangeAddresses(ctx context.Context, exchanges []*net.MX) ([]net.IP, error) {
	var addresses []net.IP
	var err error
	for index := 0; index < len(exchanges) && err == nil; index++ {
		var found []net.IP
		found, err = session.lookup.addresses(ctx, exchanges[index].Host)
		addresses = append(addresses, found...)
	}
	return addresses, err
}

func (session *spfSession) resolve(ctx context.Context, target string) ([]net.IP, error) {
	addresses, err := session.lookup.addresses(ctx, target)
	if err != nil {
		return nil, err
	}
	return addresses, session.countVoid(len(addresses))
}

func (session *spfSession) network(term spfTerm) spfNetwork {
	return spfNetwork{remote: session.remote, prefix4: term.prefix4, prefix6: term.prefix6}
}

func (session *spfSession) target(domain string, term spfTerm) (string, error) {
	err := session.countLookup()
	if err != nil {
		return "", err
	}
	return newSpfMacro(session.identity, session.remote, domain).expand(cmp.Or(term.domain, domain))
}

func (session *spfSession) countLookup() error {
	session.lookups++
	if session.lookups > session.limits.lookups() {
		return failure{result: Outcomes{}.PermError()}
	}
	return nil
}

func (session *spfSession) countVoid(answers int) error {
	session.voids += map[bool]int{true: 1}[answers == 0]
	if session.voids > session.limits.voids() {
		return failure{result: Outcomes{}.PermError()}
	}
	return nil
}

func (session *spfSession) limitExchanges(count int) error {
	if count > session.limits.exchanges() {
		return failure{result: Outcomes{}.PermError()}
	}
	return nil
}
