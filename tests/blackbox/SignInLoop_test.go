//go:build blackbox

package blackbox

import (
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"
)

type signInLoop struct {
	stack    Stack
	login    string
	password string
	within   time.Duration
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
	if reply.Status != http.StatusOK || took > loop.within {
		t.Fatalf("sign-in at %s answered %d after %s (want 200 within %s)", started.UTC().Format(time.TimeOnly), reply.Status, took.Round(time.Millisecond), loop.within)
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
	count := signInLoop{stack: stack, login: "ana", password: stack.Password(t, "ANA"), within: 2 * time.Second}.run(t)
	t.Logf("%d sign-ins, each answered 200 within 2s", count)
}

func k3Members(t *testing.T) int {
	t.Helper()
	text := os.Getenv("MESAIL_K3_MEMBERS")
	if text == "" {
		t.Skip("set MESAIL_K3_MEMBERS (for example 200) to fill a domain and remove it while signing in")
	}
	count, err := strconv.Atoi(text)
	if err != nil || count < 1 {
		t.Fatalf("MESAIL_K3_MEMBERS must be a positive number, got %q", text)
	}
	return count
}

func fillDomain(t *testing.T, stack Stack, domainId string, count int) {
	t.Helper()
	operator := operatorToken(t, stack)
	tokenTaken := time.Now()
	for index := range count {
		if time.Since(tokenTaken) > 30*time.Second {
			operator, tokenTaken = operatorToken(t, stack), time.Now()
		}
		addedInBulk(t, operatorAddReply(t, stack, operator, domainId, freshPrefix(), index == 0))
	}
}

func TestK3SignInStaysQuickWhileADomainWithManyMembersIsRemoved(t *testing.T) {
	count := k3Members(t)
	stack := NewStack(t)
	domainId := createDomain(t, stack, operatorToken(t, stack))
	started := time.Now()
	fillDomain(t, stack, domainId, count)
	t.Logf("added %d members to %s in %s", count, domainId, time.Since(started).Round(time.Second))
	stack.Call(t, http.MethodDelete, "/api/v1/directory/domains/"+domainId, operatorToken(t, stack), nil).Expect(t, http.StatusAccepted)
	signIns := signInLoop{stack: stack, login: "ana", password: stack.Password(t, "ANA"), within: 5 * time.Second}.run(t)
	t.Logf("%d sign-ins while %s was being removed, each answered 200 within 5s", signIns, domainId)
}
