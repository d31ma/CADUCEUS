package connections

import (
	"encoding/json"
	"math"
	"strconv"
	"sync"
	"time"
)

type Session struct {
	id        string
	events    chan []byte
	closing   chan struct{}
	closeOnce sync.Once
	code      int
	farewell  []byte
	mutex     sync.Mutex
	domainId  string
	deadline  *time.Timer
	reauth    *time.Timer
	lead      time.Duration
}

func NewSession(id string, settings SessionSettings) *Session {
	session := &Session{
		id:      id,
		events:  make(chan []byte, settings.QueueLimit),
		closing: make(chan struct{}),
		lead:    settings.ReauthLead,
	}
	session.reauth = session.idleTimer(session.askReauth)
	session.deadline = session.idleTimer(session.expire)
	session.deadline.Reset(settings.AuthWindow)
	session.offerOrClose(session.encoded(session.hello()))
	return session
}

func (session *Session) Id() string {
	return session.id
}

func (session *Session) DomainId() string {
	session.mutex.Lock()
	defer session.mutex.Unlock()
	return session.domainId
}

func (session *Session) Events() <-chan []byte {
	return session.events
}

func (session *Session) Closing() <-chan struct{} {
	return session.closing
}

func (session *Session) CloseCode() int {
	<-session.closing
	return session.code
}

func (session *Session) Farewell() []byte {
	<-session.closing
	return session.farewell
}

func (session *Session) Close(code int) {
	session.closeWith(code, nil)
}

func (session *Session) Accept(domainId string, expiresAt string) {
	session.keepDomain(domainId)
	session.moveDeadline(expiresAt)
}

func (session *Session) Push(event []byte) bool {
	return session.offerOrClose(event)
}

func (session *Session) closeWith(code int, farewell []byte) {
	session.closeOnce.Do(func() {
		session.code = code
		session.farewell = farewell
		session.deadline.Stop()
		session.reauth.Stop()
		close(session.closing)
	})
}

func (session *Session) expire() {
	event := RealtimeEvent{Type: "auth.expired", State: strconv.Itoa(CloseCodes{}.Unauthorized())}
	session.closeWith(CloseCodes{}.Unauthorized(), session.encoded(event))
}

func (session *Session) askReauth() {
	session.offerOrClose(session.encoded(RealtimeEvent{Type: "reauth"}))
}

func (*Session) idleTimer(action func()) *time.Timer {
	timer := time.AfterFunc(time.Duration(math.MaxInt64), action)
	timer.Stop()
	return timer
}

func (*Session) hello() RealtimeEvent {
	return RealtimeEvent{Type: "hello"}
}

func (*Session) encoded(event RealtimeEvent) []byte {
	encoded, _ := json.Marshal(event)
	return encoded
}

func (session *Session) keepDomain(domainId string) {
	if domainId == "" {
		return
	}
	session.mutex.Lock()
	defer session.mutex.Unlock()
	session.domainId = domainId
}

func (session *Session) moveDeadline(expiresAt string) {
	deadline, err := time.Parse(time.RFC3339Nano, expiresAt)
	if err != nil {
		return
	}
	session.deadline.Reset(time.Until(deadline))
	session.reauth.Reset(time.Until(deadline) - session.lead)
}

func (session *Session) offerOrClose(event []byte) bool {
	select {
	case session.events <- event:
		return true
	default:
		session.Close(CloseCodes{}.TryAgainLater())
		return false
	}
}
