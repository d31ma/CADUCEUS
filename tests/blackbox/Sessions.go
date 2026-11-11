//go:build blackbox

package blackbox

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

type session struct {
	Me                 member         `json:"me"`
	Domains            []domainChoice `json:"domains"`
	AccessToken        string         `json:"accessToken"`
	ExpiresIn          int            `json:"expiresIn"`
	SessionKey         string         `json:"sessionKey"`
	MustChangePassword bool           `json:"mustChangePassword"`
}

type domainChoice struct {
	DomainId string `json:"domainId"`
	Admin    bool   `json:"admin"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field"`
}

func (stack Stack) Password(t *testing.T, name string) string {
	t.Helper()
	password := os.Getenv("MESAIL_PASSWORD_" + name)
	if password == "" {
		password = os.Getenv("MESAIL_PASSWORD")
	}
	if password == "" {
		t.Skipf("MESAIL_PASSWORD_%s and MESAIL_PASSWORD are not set", name)
	}
	return password
}

func signInReply(t *testing.T, stack Stack, login string, password string) Reply {
	t.Helper()
	request := map[string]string{"login": login, "password": password}
	return stack.Call(t, http.MethodPost, "/api/v1/identity/session", "", request)
}

func signIn(t *testing.T, stack Stack, login string, password string) session {
	t.Helper()
	var signedIn session
	signInReply(t, stack, login, password).Expect(t, http.StatusOK).Decode(t, &signedIn)
	return signedIn
}

func fixture(t *testing.T, stack Stack, name string) session {
	t.Helper()
	return signIn(t, stack, strings.ToLower(name), stack.Password(t, name))
}

func fixtureIn(t *testing.T, stack Stack, name string, domainId string) session {
	t.Helper()
	return signIn(t, stack, strings.ToLower(name)+"@"+domainId, stack.Password(t, name))
}

func refreshReply(t *testing.T, stack Stack, sessionKey string) Reply {
	t.Helper()
	request := map[string]string{"sessionKey": sessionKey}
	return stack.Call(t, http.MethodPost, "/api/v1/identity/session/refresh", "", request)
}

func switchReply(t *testing.T, stack Stack, sessionKey string, domainId string) Reply {
	t.Helper()
	request := map[string]string{"sessionKey": sessionKey, "domainId": domainId}
	return stack.Call(t, http.MethodPost, "/api/v1/identity/session/domain", "", request)
}

func switchDomain(t *testing.T, stack Stack, sessionKey string, domainId string) session {
	t.Helper()
	var switched session
	switchReply(t, stack, sessionKey, domainId).Expect(t, http.StatusOK).Decode(t, &switched)
	return switched
}

func (reply Reply) Error(t *testing.T) apiError {
	t.Helper()
	var failure apiError
	reply.Decode(t, &failure)
	return failure
}

func (reply Reply) ExpectError(t *testing.T, status int, code string) {
	t.Helper()
	failure := reply.Expect(t, status).Error(t)
	if failure.Code != code {
		t.Fatalf("want error code %s, got %+v", code, failure)
	}
}

func freshPrefix() string {
	return "bb" + strconv.FormatInt(time.Now().UnixNano(), 36)
}

func firstPassword(prefix string) string {
	return "Blackbox-" + prefix + "-First-1"
}

func tokenPart(t *testing.T, token string, index int) map[string]any {
	t.Helper()
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("not a JWT: %d parts", len(parts))
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[index])
	if err != nil {
		t.Fatal(err)
	}
	decoded := map[string]any{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded
}

func tokenHeader(t *testing.T, token string) map[string]any {
	t.Helper()
	return tokenPart(t, token, 0)
}

func tokenClaims(t *testing.T, token string) map[string]any {
	t.Helper()
	return tokenPart(t, token, 1)
}

func domainIds(choices []domainChoice) []string {
	ids := []string{}
	for _, choice := range choices {
		ids = append(ids, choice.DomainId)
	}
	return ids
}
