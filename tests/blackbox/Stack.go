//go:build blackbox

package blackbox

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

type Stack struct {
	baseUrl string
	client  *http.Client
}

type Reply struct {
	Status int
	Body   []byte
}

func NewStack(t *testing.T) Stack {
	t.Helper()
	baseUrl := os.Getenv("MESAIL_BASE_URL")
	if baseUrl == "" {
		t.Skip("MESAIL_BASE_URL is not set; blackbox tests need a running Mesail")
	}
	return Stack{baseUrl: baseUrl, client: &http.Client{Timeout: 10 * time.Second}}
}

func (stack Stack) Token(t *testing.T, name string) string {
	t.Helper()
	token := os.Getenv("MESAIL_TOKEN_" + name)
	if token == "" {
		t.Skipf("MESAIL_TOKEN_%s is not set", name)
	}
	return token
}

func (stack Stack) Call(t *testing.T, method string, path string, token string, body any) Reply {
	t.Helper()
	request, err := http.NewRequest(method, stack.baseUrl+path, stack.encode(t, body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := stack.client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return Reply{Status: response.StatusCode, Body: raw}
}

func (stack Stack) encode(t *testing.T, body any) io.Reader {
	t.Helper()
	if body == nil {
		return http.NoBody
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(raw)
}

func (reply Reply) Decode(t *testing.T, into any) {
	t.Helper()
	if err := json.Unmarshal(reply.Body, into); err != nil {
		t.Fatalf("status %d, body %s: %v", reply.Status, reply.Body, err)
	}
}

func (reply Reply) Expect(t *testing.T, status int) Reply {
	t.Helper()
	if reply.Status != status {
		t.Fatalf("want status %d, got %d: %s", status, reply.Status, reply.Body)
	}
	return reply
}

func (stack Stack) Eventually(t *testing.T, within time.Duration, what string, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(within)
	for !check() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out after %s waiting for %s", within, what)
		}
		time.Sleep(time.Second)
	}
}
