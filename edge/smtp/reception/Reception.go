package reception

import (
	"context"
	"net"
)

type Reception struct {
	parts Parts
}

func NewReception(parts Parts) Reception {
	return Reception{parts: parts}
}

func (reception Reception) Deliver(ctx context.Context, transaction Transaction) (Receipt, error) {
	verdict := reception.parts.Checker.Check(ctx, reception.authInput(transaction))
	if err := reception.refusal(verdict); err != nil {
		return Receipt{}, err
	}
	message := PreparedMessage{Transaction: transaction, Verdict: verdict, Headers: reception.headers(transaction.Raw)}
	return reception.deliverAll(ctx, message, DomainGroups{}.ByDomain(transaction.Recipients))
}

func (reception Reception) authInput(transaction Transaction) AuthInput {
	return AuthInput{
		RemoteIp:   reception.remoteIp(transaction.RemoteAddress),
		Helo:       transaction.Helo,
		MailFrom:   transaction.EnvelopeFrom,
		Raw:        transaction.Raw,
		ReceivedAt: transaction.ReceivedAt,
	}
}

func (Reception) remoteIp(address string) net.IP {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return net.ParseIP(address)
	}
	return net.ParseIP(host)
}

func (Reception) refusal(verdict AuthVerdict) error {
	if verdict.Reject {
		return Refused{}
	}
	return nil
}

func (reception Reception) headers(raw []byte) InboundHeaders {
	headers, err := reception.parts.Headers.Headers(raw)
	if err != nil {
		return InboundHeaders{References: []string{}, To: []MailboxAddress{}, Cc: []MailboxAddress{}}
	}
	return headers
}

func (reception Reception) deliverAll(ctx context.Context, message PreparedMessage, groups []DomainGroup) (Receipt, error) {
	receipt := Receipt{InboundIds: []string{}}
	for _, group := range groups {
		inboundId, err := reception.deliverGroup(ctx, message, group)
		if err != nil {
			return Receipt{}, err
		}
		receipt.InboundIds = append(receipt.InboundIds, inboundId)
	}
	return receipt, nil
}

func (reception Reception) deliverGroup(ctx context.Context, message PreparedMessage, group DomainGroup) (string, error) {
	key := RawKey{}.For(message.Transaction.Raw, message.Transaction.EnvelopeFrom, group)
	content := message.StoredContent()
	if err := reception.parts.Store.Put(ctx, key, content); err != nil {
		return "", err
	}
	return reception.parts.MailIn.Accept(ctx, message.Envelope(group, key, len(content)))
}
