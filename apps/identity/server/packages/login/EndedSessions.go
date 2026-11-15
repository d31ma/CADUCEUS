package login

import "time"

type EndedSessions struct{}

func (sessions EndedSessions) Check(person Person, signedInAt string) error {
	return sessions.refused(sessions.ended(person.SessionsEndedAt, signedInAt))
}

func (sessions EndedSessions) ended(endedAt string, signedInAt string) bool {
	if endedAt == "" {
		return false
	}
	return sessions.startedBefore(signedInAt, endedAt)
}

func (EndedSessions) startedBefore(signedInAt string, endedAt string) bool {
	started, _ := time.Parse(time.RFC3339Nano, signedInAt)
	ended, err := time.Parse(time.RFC3339Nano, endedAt)
	return err != nil || started.Before(ended)
}

func (EndedSessions) refused(ended bool) error {
	if ended {
		return AccessRevoked{}
	}
	return nil
}
