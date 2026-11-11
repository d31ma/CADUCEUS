package connections

import (
	"context"
	"sync"
)

type Hub struct {
	mutex        sync.Mutex
	idle         *sync.Cond
	sessions     map[string]*Session
	running      int
	shutdownCode int
}

func NewHub() *Hub {
	hub := &Hub{sessions: make(map[string]*Session)}
	hub.idle = sync.NewCond(&hub.mutex)
	return hub
}

func (hub *Hub) Add(session *Session) {
	hub.closeWhenShuttingDown(session, hub.track(session))
}

func (hub *Hub) Remove(session *Session) {
	hub.mutex.Lock()
	defer hub.mutex.Unlock()
	delete(hub.sessions, session.Id())
}

func (hub *Hub) Finished() {
	hub.mutex.Lock()
	defer hub.mutex.Unlock()
	hub.running--
	hub.idle.Broadcast()
}

func (hub *Hub) Find(connectionId string) (*Session, bool) {
	hub.mutex.Lock()
	defer hub.mutex.Unlock()
	session, found := hub.sessions[connectionId]
	return session, found
}

func (hub *Hub) CloseAll(ctx context.Context, code int) error {
	hub.closeEach(hub.snapshot(code), code)
	return hub.wait(ctx)
}

func (hub *Hub) track(session *Session) int {
	hub.mutex.Lock()
	defer hub.mutex.Unlock()
	hub.sessions[session.Id()] = session
	hub.running++
	return hub.shutdownCode
}

func (hub *Hub) closeWhenShuttingDown(session *Session, shutdownCode int) {
	if shutdownCode == 0 {
		return
	}
	session.Close(shutdownCode)
}

func (hub *Hub) snapshot(code int) []*Session {
	hub.mutex.Lock()
	defer hub.mutex.Unlock()
	hub.shutdownCode = code
	sessions := make([]*Session, 0, len(hub.sessions))
	for _, session := range hub.sessions {
		sessions = append(sessions, session)
	}
	return sessions
}

func (hub *Hub) closeEach(sessions []*Session, code int) {
	for _, session := range sessions {
		session.Close(code)
	}
}

func (hub *Hub) wait(ctx context.Context) error {
	done := make(chan struct{})
	go hub.signalWhenIdle(done)
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (hub *Hub) signalWhenIdle(done chan<- struct{}) {
	hub.mutex.Lock()
	for hub.running > 0 {
		hub.idle.Wait()
	}
	hub.mutex.Unlock()
	close(done)
}
