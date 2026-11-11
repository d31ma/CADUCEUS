//go:build blackbox

package blackbox

import (
	"net/http"
	"slices"
	"testing"
)

type channel struct {
	ChannelId  string `json:"channelId"`
	Name       string `json:"name"`
	Visibility string `json:"visibility"`
}

func createPublicChannel(t *testing.T, stack Stack, token string, name string) string {
	t.Helper()
	var created channel
	request := map[string]string{"name": name, "topic": "blackbox test channel", "visibility": "public"}
	stack.Call(t, http.MethodPost, "/api/v1/channels", token, request).Expect(t, http.StatusCreated).Decode(t, &created)
	if created.ChannelId == "" {
		t.Fatalf("created channel has no channelId: %+v", created)
	}
	return created.ChannelId
}

func visibleChannelIds(t *testing.T, stack Stack, token string) []string {
	t.Helper()
	var channels []channel
	stack.Call(t, http.MethodGet, "/api/v1/channels", token, nil).Expect(t, http.StatusOK).Decode(t, &channels)
	ids := []string{}
	for _, listed := range channels {
		ids = append(ids, listed.ChannelId)
	}
	return ids
}

func TestPublicChannelIsVisibleOnlyInsideItsDomain(t *testing.T) {
	stack := NewStack(t)
	ana := stack.Token(t, "ANA")
	bo := stack.Token(t, "BO")
	eve := stack.Token(t, "EVE")
	channelId := createPublicChannel(t, stack, ana, uniqueClientId("release"))
	if !slices.Contains(visibleChannelIds(t, stack, bo), channelId) {
		t.Fatalf("Bo in the same domain does not see channel %s", channelId)
	}
	if slices.Contains(visibleChannelIds(t, stack, eve), channelId) {
		t.Fatalf("Eve in another domain sees channel %s", channelId)
	}
	stack.Call(t, http.MethodPost, "/api/v1/channels/"+channelId+"/join", eve, nil).Expect(t, http.StatusNotFound)
}
