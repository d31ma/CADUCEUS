package realtime

import (
	"encoding/json"
	"errors"
	"time"

	"mesail.dev/packages/events"
	"mesail.dev/packages/oidc"
)

type presenceVerifier struct {
	claims map[string]oidc.Claims
}

func (verifier presenceVerifier) Verify(rawToken string, now time.Time) (oidc.Claims, error) {
	claims, found := verifier.claims[rawToken]
	if !found {
		return oidc.Claims{}, errors.New("refused")
	}
	return claims, nil
}

func presenceNow() time.Time {
	return time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
}

func presenceText(offset time.Duration) string {
	return presenceNow().Add(offset).Format(time.RFC3339)
}

func presenceClaims(domainId string, memberId string, realmRoles ...string) oidc.Claims {
	return oidc.Claims{Issuer: "https://id.mesail.test/realms/" + domainId, Subject: memberId, RealmRoles: realmRoles}
}

func presenceGate() oidc.Gate {
	verifier := presenceVerifier{claims: map[string]oidc.Claims{
		"ana":       presenceClaims("example.com", "ana", "mesail-member"),
		"service":   presenceClaims("example.com", "service-account", "mesail-service"),
		"elsewhere": presenceClaims("example.org", "service-account", "mesail-service"),
		"operators": presenceClaims("mesail-operators", "service-account", "mesail-service"),
		"operator":  presenceClaims("mesail-operators", "olga", "mesail-operator"),
	}}
	return oidc.NewGate(verifier, presenceNow)
}

func presenceKeeper(memory *memoryStorage, gateway *recordingGateway) PresenceKeeper {
	connections := ConnectionBook{Storage: memory.storage(), Now: presenceNow}
	return PresenceKeeper{Book: PresenceBook{Storage: memory.storage()}, Connections: connections, FanOut: FanOut{Connections: connections, Gateway: gateway}, Now: presenceNow}
}

func presenceEventDesk(memory *memoryStorage, gateway *recordingGateway) EventDesk {
	connections := ConnectionBook{Storage: memory.storage(), Now: presenceNow}
	return EventDesk{Gate: presenceGate(), Seen: SeenBook{Storage: memory.storage()}, Members: MemberBook{Storage: memory.storage()}, FanOut: FanOut{Connections: connections, Gateway: gateway}}
}

func presenceEnvelope(eventId string, eventType string, domainId string, payload any) events.Envelope {
	producer, _ := events.Catalog{}.ProducerOf(eventType)
	raw, _ := json.Marshal(payload)
	return events.Envelope{EventID: eventId, Type: eventType, DomainId: domainId, Producer: producer, OccurredAt: presenceText(0), Payload: raw}
}
