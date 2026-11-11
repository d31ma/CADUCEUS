//go:build blackbox

package blackbox

import (
	"net/http"
	"net/url"
	"testing"
	"time"
)

func searchHitCount(t *testing.T, stack Stack, token string, text string) int {
	t.Helper()
	var page struct {
		Items []struct {
			MessageId string `json:"messageId"`
		} `json:"items"`
	}
	path := "/api/v1/search?q=" + url.QueryEscape(text)
	stack.Call(t, http.MethodGet, path, token, nil).Expect(t, http.StatusOK).Decode(t, &page)
	return len(page.Items)
}

func TestAcSr2SearchNeverShowsAnotherDomainsMessages(t *testing.T) {
	stack := NewStack(t)
	ana := stack.Token(t, "ANA")
	eve := stack.Token(t, "EVE")
	text := uniqueClientId("searchable")
	send(t, stack, ana, toOne("search", text, "bo@example.com", "chat", "member"), http.StatusCreated)
	stack.Eventually(t, 60*time.Second, "Ana's own search to find her message", func() bool {
		return searchHitCount(t, stack, ana, text) > 0
	})
	if hits := searchHitCount(t, stack, eve, text); hits != 0 {
		t.Fatalf("leaked %d hits to another domain", hits)
	}
}
