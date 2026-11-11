package realtime

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io"
	"time"
)

type TicketBook struct {
	Storage Storage
	Now     func() time.Time
	Random  io.Reader
}

func (book TicketBook) Issue(grant Grant) (Ticket, error) {
	ticket, err := book.newTicket()
	if err != nil {
		return Ticket{}, err
	}
	err = book.keeper().Ensure(book.collection())
	if err != nil {
		return Ticket{}, err
	}
	expiresAt := book.Now().UTC().Add(book.lifetime()).Format(time.RFC3339)
	record := TicketRecord{TicketHash: book.hash(ticket), DomainId: grant.DomainId, MemberId: grant.MemberId, ExpiresAt: expiresAt, TokenExpiresAt: grant.ExpiresAt}
	_, err = book.Storage.Documents.Insert(book.collection(), record)
	if err != nil {
		return Ticket{}, Unavailable{Operation: "ticket insert"}
	}
	return Ticket{Ticket: ticket, ExpiresAt: expiresAt}, nil
}

func (book TicketBook) Redeem(ticket string) (Grant, error) {
	record, err := book.find(book.hash(ticket))
	if err != nil {
		return Grant{}, err
	}
	now := book.Now().UTC()
	err = book.usable(record, now)
	if err != nil {
		return Grant{}, err
	}
	_, err = book.Storage.Documents.Patch(book.collection(), record.ID, TicketUse{UsedAt: now.Format(time.RFC3339Nano)}, nil)
	if err != nil {
		return Grant{}, Unavailable{Operation: "ticket use"}
	}
	return Grant{DomainId: record.DomainId, MemberId: record.MemberId, ExpiresAt: record.TokenExpiresAt}, nil
}

func (book TicketBook) newTicket() (string, error) {
	random := make([]byte, book.randomBytes())
	_, err := io.ReadFull(book.Random, random)
	if err != nil {
		return "", Unavailable{Operation: "ticket random"}
	}
	return base64.RawURLEncoding.EncodeToString(random), nil
}

func (book TicketBook) find(ticketHash string) (TicketRecord, error) {
	exists, err := book.keeper().Exists(book.collection())
	if err != nil || !exists {
		return TicketRecord{}, book.unknown(err)
	}
	found := []TicketRecord{}
	_, err = book.Storage.Page(Query{Collection: book.collection(), Where: map[string]any{"ticketHash": ticketHash}, Size: 1}, &found)
	if err != nil {
		return TicketRecord{}, Unavailable{Operation: "ticket find"}
	}
	return book.first(found)
}

func (TicketBook) unknown(err error) error {
	if err != nil {
		return err
	}
	return Unauthenticated{}
}

func (TicketBook) first(found []TicketRecord) (TicketRecord, error) {
	if len(found) == 0 {
		return TicketRecord{}, Unauthenticated{}
	}
	return found[0], nil
}

func (book TicketBook) usable(record TicketRecord, now time.Time) error {
	if record.UsedAt != "" || !now.Before(book.moment(record.ExpiresAt)) || !now.Before(book.moment(record.TokenExpiresAt)) {
		return Unauthenticated{}
	}
	return nil
}

func (TicketBook) moment(text string) time.Time {
	moment, _ := time.Parse(time.RFC3339, text)
	return moment
}

func (TicketBook) hash(ticket string) string {
	digest := sha256.Sum256([]byte(ticket))
	return hex.EncodeToString(digest[:])
}

func (book TicketBook) keeper() CollectionKeeper {
	return CollectionKeeper{Documents: book.Storage.Documents}
}

func (TicketBook) collection() string {
	return "realtime-tickets"
}

func (TicketBook) lifetime() time.Duration {
	return 30 * time.Second
}

func (TicketBook) randomBytes() int {
	return 32
}
