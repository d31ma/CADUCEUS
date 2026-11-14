//go:build blackbox

package blackbox

import (
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

type removalTurns struct {
	stack    Stack
	operator string
	taken    time.Time
	started  time.Time
	gone     map[string]time.Duration
}

func (turns *removalTurns) freshOperator(t *testing.T) string {
	t.Helper()
	if time.Since(turns.taken) > 30*time.Second {
		turns.operator, turns.taken = operatorToken(t, turns.stack), time.Now()
	}
	return turns.operator
}

func (turns *removalTurns) remove(t *testing.T, domainId string) {
	t.Helper()
	turns.stack.Call(t, http.MethodDelete, "/api/v1/directory/domains/"+domainId, turns.freshOperator(t), nil).Expect(t, http.StatusAccepted)
}

func (turns *removalTurns) check(t *testing.T, domainId string, within time.Duration) {
	t.Helper()
	if _, done := turns.gone[domainId]; done {
		return
	}
	waited := time.Since(turns.started)
	if turns.stack.Call(t, http.MethodGet, "/api/v1/directory/domains/"+domainId, turns.freshOperator(t), nil).Status == http.StatusNotFound {
		turns.gone[domainId] = waited
		t.Logf("%s was gone after %s", domainId, waited.Round(time.Second))
		return
	}
	if waited > within {
		t.Fatalf("%s is still there after %s (want gone within %s)", domainId, waited.Round(time.Second), within)
	}
}

func (turns *removalTurns) addOnce(t *testing.T, domainId string, username string, admin bool) (Reply, bool) {
	t.Helper()
	path := "/api/v1/directory/domains/" + domainId + "/members"
	request, err := http.NewRequest(http.MethodPost, turns.stack.baseUrl+path, turns.stack.encode(t, addMemberRequest(username, admin)))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+turns.freshOperator(t))
	response, err := turns.stack.client.Do(request)
	var timeout net.Error
	if errors.As(err, &timeout) && timeout.Timeout() {
		return Reply{}, true
	}
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return Reply{Status: response.StatusCode, Body: raw}, false
}

func (turns *removalTurns) addWithOneRetry(t *testing.T, domainId string, username string, admin bool) bool {
	t.Helper()
	reply, timedOut := turns.addOnce(t, domainId, username, admin)
	if !timedOut {
		reply.Expect(t, http.StatusCreated)
		return false
	}
	t.Logf("retry: adding %s to %s timed out at %s; trying once more (K7 and K10 remove this retry)", username, domainId, time.Now().UTC().Format(time.TimeOnly))
	reply, timedOut = turns.addOnce(t, domainId, username, admin)
	if timedOut {
		t.Fatalf("adding %s to %s timed out twice", username, domainId)
	}
	if reply.Status == http.StatusConflict && strings.Contains(string(reply.Body), "duplicate") {
		return true
	}
	reply.Expect(t, http.StatusCreated)
	return true
}

func (turns *removalTurns) fill(t *testing.T, domainId string, count int) {
	t.Helper()
	retries := 0
	for index := range count {
		if turns.addWithOneRetry(t, domainId, freshPrefix(), index == 0) {
			retries++
		}
	}
	t.Logf("filled %s with %d members; %d adds needed a retry", domainId, count, retries)
	if retries*20 > count {
		t.Fatalf("%d of %d adds needed a retry (more than 5%%)", retries, count)
	}
}

func TestK6ASmallDomainIsGoneSoonWhileABigOneIsRemovedInMinutes(t *testing.T) {
	count := k3Members(t)
	stack := NewStack(t)
	turns := &removalTurns{stack: stack, gone: map[string]time.Duration{}}
	big, small := createDomain(t, stack, turns.freshOperator(t)), createDomain(t, stack, turns.freshOperator(t))
	turns.fill(t, big, count)
	turns.fill(t, small, 1)
	turns.remove(t, big)
	turns.remove(t, small)
	turns.started = time.Now()
	loop := signInLoop{stack: stack, login: "ana", password: stack.Password(t, "ANA"), within: 5 * time.Second}
	signIns := 0
	for len(turns.gone) < 2 {
		loop.signInOnce(t)
		signIns++
		turns.check(t, small, 2*time.Minute)
		turns.check(t, big, 5*time.Minute)
	}
	t.Logf("%d sign-ins while the domains were removed, each answered 200 within 5s", signIns)
}
