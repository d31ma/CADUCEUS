package connections

import (
	"encoding/json"
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
}

func NewSession(id string, settings SessionSettings) *Session {
	session := &Session{
		id:      id,
		events:  make(chan []byte, settings.QueueLimit),
		closing: make(chan struct{}),
	}
	session.deadline = time.AfterFunc(settings.AuthWindow, session.expire)
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
		close(session.closing)
	})
}

func (session *Session) expire() {
	event := RealtimeEvent{Type: "auth.expired", State: strconv.Itoa(CloseCodes{}.Unauthorized())}
	encoded, _ := json.Marshal(event)
	session.closeWith(CloseCodes{}.Unauthorized(), encoded)
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
