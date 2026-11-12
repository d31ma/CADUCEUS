//go:build blackbox

package blackbox

import (
	"net/http"
	"os"
	"testing"
)

func keepTestData() bool {
	return os.Getenv("MESAIL_KEEP_TEST_DATA") != ""
}

func cleanupToken(t *testing.T, stack Stack) string {
	t.Helper()
	password := os.Getenv("MESAIL_PASSWORD_OPERATOR")
	if password == "" {
		password = os.Getenv("MESAIL_PASSWORD")
	}
	reply := signInReply(t, stack, "operator", password)
	if reply.Status != http.StatusOK {
		t.Logf("clean-up skipped: the operator could not sign in (status %d)", reply.Status)
		return ""
	}
	var signedIn session
	reply.Decode(t, &signedIn)
	return signedIn.AccessToken
}

func cleanupDelete(t *testing.T, stack Stack, path string) {
	t.Helper()
	token := cleanupToken(t, stack)
	if token == "" {
		return
	}
	reply := stack.Call(t, http.MethodDelete, path, token, nil)
	if reply.Status >= 300 && reply.Status != http.StatusNotFound {
		t.Logf("clean-up of %s answered %d: %s", path, reply.Status, reply.Body)
	}
}

func removeAfterTest(t *testing.T, stack Stack, path string) {
	t.Helper()
	if keepTestData() {
		return
	}
	t.Cleanup(func() { cleanupDelete(t, stack, path) })
}
