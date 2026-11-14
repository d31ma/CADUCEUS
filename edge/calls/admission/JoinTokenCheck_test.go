package admission_test

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mesail.dev/edge/calls/admission"
	"mesail.dev/edge/calls/failures"
	"mesail.dev/edge/calls/fakerealtime"
	"mesail.dev/packages/oidc"
)

type fixedClock struct {
	now time.Time
}

func (clock *fixedClock) Now() time.Time {
	return clock.now
}

type countingFetcher struct {
	set   oidc.KeySet
	err   error
	calls int
}

func (fetcher *countingFetcher) Fetch(url string) (oidc.KeySet, error) {
	fetcher.calls++
	return fetcher.set, fetcher.err
}

func newKeys(t *testing.T) fakerealtime.JoinKeys {
	keys, err := fakerealtime.JoinKeys{}.Generated()
	if err != nil {
		t.Fatal(err)
	}
	return keys
}

func newSet(t *testing.T, keys fakerealtime.JoinKeys) oidc.KeySet {
	set, err := keys.Set()
	if err != nil {
		t.Fatal(err)
	}
	return set
}

func token(t *testing.T, keys fakerealtime.JoinKeys, claims admission.JoinClaims) string {
	text, err := keys.Token(claims)
	if err != nil {
		t.Fatal(err)
	}
	return text
}

func goodClaims(now time.Time) admission.JoinClaims {
	return admission.JoinClaims{CallId: "call-1", DeviceId: "device-1", DomainId: "example.com", Audience: "calls", ExpiresAt: now.Add(time.Minute).Unix(), IssuedAt: now.Unix()}
}

func newCheck(t *testing.T, keys fakerealtime.JoinKeys, clock *fixedClock) admission.JoinTokenCheck {
	fetcher := &countingFetcher{set: newSet(t, keys)}
	return admission.NewJoinTokenCheck(admission.NewKeyBook(fetcher, "http://realtime/keys", clock), clock)
}

func number(err error) int {
	return failures.Problems{}.From(err).Number()
}

func TestAdmitAcceptsAGoodToken(t *testing.T) {
	clock := &fixedClock{now: time.Unix(1_800_000_000, 0)}
	keys := newKeys(t)
	claims, err := newCheck(t, keys, clock).Admit(token(t, keys, goodClaims(clock.now)), "call-1")
	if err != nil {
		t.Fatal(err)
	}
	if claims.DeviceId != "device-1" || claims.DomainId != "example.com" {
		t.Fatalf("claims %+v", claims)
	}
}

func TestAdmitRefusesAnExpiredToken(t *testing.T) {
	clock := &fixedClock{now: time.Unix(1_800_000_000, 0)}
	keys := newKeys(t)
	raw := token(t, keys, goodClaims(clock.now))
	clock.now = clock.now.Add(61 * time.Second)
	_, err := newCheck(t, keys, clock).Admit(raw, "call-1")
	if number(err) != 6503 {
		t.Fatalf("error %v", err)
	}
}

func TestIdentifyAcceptsAnExpiredToken(t *testing.T) {
	clock := &fixedClock{now: time.Unix(1_800_000_000, 0)}
	keys := newKeys(t)
	raw := token(t, keys, goodClaims(clock.now))
	clock.now = clock.now.Add(time.Hour)
	_, err := newCheck(t, keys, clock).Identify(raw, "call-1")
	if err != nil {
		t.Fatal(err)
	}
}

func TestAdmitRefusesAMissingToken(t *testing.T) {
	clock := &fixedClock{now: time.Unix(1_800_000_000, 0)}
	_, err := newCheck(t, newKeys(t), clock).Admit("", "call-1")
	if number(err) != 6501 {
		t.Fatalf("error %v", err)
	}
}

func TestAdmitRefusesAnotherCall(t *testing.T) {
	clock := &fixedClock{now: time.Unix(1_800_000_000, 0)}
	keys := newKeys(t)
	_, err := newCheck(t, keys, clock).Admit(token(t, keys, goodClaims(clock.now)), "call-2")
	if number(err) != 6504 {
		t.Fatalf("error %v", err)
	}
}

func TestAdmitRefusesATokenSignedByAnotherKey(t *testing.T) {
	clock := &fixedClock{now: time.Unix(1_800_000_000, 0)}
	trusted := newKeys(t)
	stranger := newKeys(t)
	_, err := newCheck(t, trusted, clock).Admit(token(t, stranger, goodClaims(clock.now)), "call-1")
	if number(err) != 6502 {
		t.Fatalf("error %v", err)
	}
}

func TestAdmitRefusesAChangedPayload(t *testing.T) {
	clock := &fixedClock{now: time.Unix(1_800_000_000, 0)}
	keys := newKeys(t)
	parts := strings.Split(token(t, keys, goodClaims(clock.now)), ".")
	other := strings.Split(token(t, keys, admission.JoinClaims{CallId: "call-1", DeviceId: "device-9", DomainId: "example.com", Audience: "calls", ExpiresAt: clock.now.Add(time.Hour).Unix()}), ".")
	_, err := newCheck(t, keys, clock).Admit(parts[0]+"."+other[1]+"."+parts[2], "call-1")
	if number(err) != 6502 {
		t.Fatalf("error %v", err)
	}
}

func TestAdmitRefusesBadShapes(t *testing.T) {
	clock := &fixedClock{now: time.Unix(1_800_000_000, 0)}
	keys := newKeys(t)
	check := newCheck(t, keys, clock)
	cases := []string{"not-a-token", "a.b.c", token(t, keys, admission.JoinClaims{CallId: "call-1", DeviceId: "device-1", DomainId: "example.com", Audience: "realtime", ExpiresAt: clock.now.Add(time.Minute).Unix()}), token(t, keys, admission.JoinClaims{CallId: "call-1", Audience: "calls", ExpiresAt: clock.now.Add(time.Minute).Unix()})}
	for _, raw := range cases {
		_, err := check.Admit(raw, "call-1")
		if number(err) != 6502 {
			t.Fatalf("token %q gave %v", raw, err)
		}
	}
}

func TestClaimsThatAreNotJsonAreRefused(t *testing.T) {
	clock := &fixedClock{now: time.Unix(1_800_000_000, 0)}
	keys := newKeys(t)
	parts := strings.Split(token(t, keys, goodClaims(clock.now)), ".")
	_, err := newCheck(t, keys, clock).Admit(parts[0]+".bm90IGpzb24."+parts[2], "call-1")
	if number(err) != 6502 {
		t.Fatalf("error %v", err)
	}
}

func TestKeyBookFetchesOnceAndRetriesAfterTheInterval(t *testing.T) {
	clock := &fixedClock{now: time.Unix(1_800_000_000, 0)}
	keys := newKeys(t)
	fetcher := &countingFetcher{set: newSet(t, keys)}
	book := admission.NewKeyBook(fetcher, "http://realtime/keys", clock)
	check := admission.NewJoinTokenCheck(book, clock)
	raw := token(t, keys, goodClaims(clock.now))
	_, _ = check.Admit(raw, "call-1")
	_, _ = check.Admit(raw, "call-1")
	_, _ = book.Key("unknown")
	if fetcher.calls != 1 {
		t.Fatalf("fetched %d times", fetcher.calls)
	}
	clock.now = clock.now.Add(11 * time.Second)
	_, _ = book.Key("unknown")
	if fetcher.calls != 2 {
		t.Fatalf("fetched %d times", fetcher.calls)
	}
}

func TestKeyBookReportsUnavailableKeysAndKeepsOldKeys(t *testing.T) {
	clock := &fixedClock{now: time.Unix(1_800_000_000, 0)}
	keys := newKeys(t)
	fetcher := &countingFetcher{err: errors.New("down")}
	book := admission.NewKeyBook(fetcher, "http://realtime/keys", clock)
	raw := token(t, keys, goodClaims(clock.now))
	_, err := admission.NewJoinTokenCheck(book, clock).Admit(raw, "call-1")
	if number(err) != 6512 {
		t.Fatalf("error %v", err)
	}
	fetcher.err = nil
	fetcher.set = newSet(t, keys)
	clock.now = clock.now.Add(11 * time.Second)
	_, err = admission.NewJoinTokenCheck(book, clock).Admit(token(t, keys, goodClaims(clock.now)), "call-1")
	if err != nil {
		t.Fatal(err)
	}
	fetcher.err = errors.New("down again")
	clock.now = clock.now.Add(11 * time.Minute)
	_, err = admission.NewJoinTokenCheck(book, clock).Admit(token(t, keys, goodClaims(clock.now)), "call-1")
	if err != nil {
		t.Fatalf("old keys were not kept: %v", err)
	}
}

func TestKeyBookReadsTheFakeRealtimeOverHttp(t *testing.T) {
	keys := newKeys(t)
	server := httptest.NewServer(keys)
	defer server.Close()
	clock := &fixedClock{now: time.Now()}
	book := admission.NewKeyBook(oidc.HttpKeyFetcher{Client: server.Client()}, server.URL, clock)
	_, err := admission.NewJoinTokenCheck(book, admission.SystemClock{}).Admit(token(t, keys, goodClaims(time.Now())), "call-1")
	if err != nil {
		t.Fatal(err)
	}
}
