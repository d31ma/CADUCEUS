//go:build blackbox

package blackbox

import (
	"net/http"
	"os"
	"testing"
	"time"
)

type signInLoop struct {
	stack    Stack
	login    string
	password string
}

func (loop signInLoop) quickest() time.Duration {
	return 2 * time.Second
}

func (loop signInLoop) duration(t *testing.T) time.Duration {
	t.Helper()
	text := os.Getenv("MESAIL_SIGNIN_LOOP")
	if text == "" {
		return 20 * time.Second
	}
	duration, err := time.ParseDuration(text)
	if err != nil {
		t.Fatalf("MESAIL_SIGNIN_LOOP must be a Go duration such as 240s, got %q", text)
	}
	return duration
}

func (loop signInLoop) signInOnce(t *testing.T) {
	t.Helper()
	started := time.Now()
	reply := signInReply(t, loop.stack, loop.login, loop.password)
	took := time.Since(started)
	if reply.Status != http.StatusOK || took > loop.quickest() {
		t.Fatalf("sign-in at %s answered %d after %s (want 200 within %s)", started.UTC().Format(time.TimeOnly), reply.Status, took.Round(time.Millisecond), loop.quickest())
	}
}

func (loop signInLoop) run(t *testing.T) int {
	t.Helper()
	deadline := time.Now().Add(loop.duration(t))
	count := 0
	for time.Now().Before(deadline) {
		loop.signInOnce(t)
		count++
	}
	return count
}

func TestK2SignInStaysQuickWhileAPersonIsDeleted(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	dan := createPerson(t, stack, ana.AccessToken)
	stack.Call(t, http.MethodDelete, "/api/v1/directory/people/"+dan.Id, operatorToken(t, stack), nil).Expect(t, http.StatusAccepted)
	count := signInLoop{stack: stack, login: "ana", password: stack.Password(t, "ANA")}.run(t)
	t.Logf("%d sign-ins, each answered 200 within 2s", count)
}
