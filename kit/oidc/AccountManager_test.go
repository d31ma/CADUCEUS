package oidc

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type accountRealm struct {
	server        *httptest.Server
	mutex         sync.Mutex
	password      string
	otpSecret     string
	sessions      []string
	ended         []string
	signOuts      int
	passwordGrant int
}

func newAccountRealm(t *testing.T) *accountRealm {
	t.Helper()
	signer := newTestSigner(t)
	realm := &accountRealm{password: "right-password", sessions: []string{"session-a", "session-b"}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /realms/example.com/certs", func(writer http.ResponseWriter, request *http.Request) {
		_ = json.NewEncoder(writer).Encode(signer.keySet())
	})
	mux.HandleFunc("POST /realms/example.com/protocol/openid-connect/token", func(writer http.ResponseWriter, request *http.Request) {
		realm.mutex.Lock()
		defer realm.mutex.Unlock()
		_ = request.ParseForm()
		form := request.PostForm
		if form.Get("grant_type") == "client_credentials" && form.Get("client_id") == "mesail-account" && form.Get("client_secret") == "account-secret" {
			_ = json.NewEncoder(writer).Encode(map[string]any{"access_token": "admin-token", "expires_in": 60})
			return
		}
		realm.passwordGrant++
		codeOk := realm.otpSecret == "" || (Totp{}).Matches(realm.otpSecret, form.Get("totp"), time.Now())
		if form.Get("client_id") != "mesail-identity" || form.Get("username") != "ana@example.com" || form.Get("password") != realm.password || !codeOk {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		claims := claimsWith("exp", time.Now().Unix()+60)
		claims["sid"] = "session-a"
		_ = json.NewEncoder(writer).Encode(map[string]any{"access_token": signer.rsaToken(t, "rsa-1", claims), "refresh_token": "refresh-a", "expires_in": 60})
	})
	mux.HandleFunc("POST /realms/example.com/protocol/openid-connect/logout", func(writer http.ResponseWriter, request *http.Request) {
		realm.mutex.Lock()
		defer realm.mutex.Unlock()
		realm.signOuts++
		writer.WriteHeader(http.StatusNoContent)
	})
	admin := http.NewServeMux()
	admin.HandleFunc("PUT /admin/realms/example.com/users/member-1/reset-password", func(writer http.ResponseWriter, request *http.Request) {
		reset := PasswordReset{}
		_ = json.NewDecoder(request.Body).Decode(&reset)
		if len(reset.Value) < 12 || reset.Temporary {
			writer.WriteHeader(http.StatusBadRequest)
			_, _ = writer.Write([]byte(`{"error":"invalidPasswordMinLengthMessage","error_description":"Invalid password: minimum length 12."}`))
			return
		}
		realm.password = reset.Value
		writer.WriteHeader(http.StatusNoContent)
	})
	admin.HandleFunc("GET /admin/realms/example.com/users/member-1/sessions", func(writer http.ResponseWriter, request *http.Request) {
		sessions := []UserSession{}
		for _, id := range realm.sessions {
			sessions = append(sessions, UserSession{Id: id})
		}
		_ = json.NewEncoder(writer).Encode(sessions)
	})
	admin.HandleFunc("DELETE /admin/realms/example.com/sessions/{id}", func(writer http.ResponseWriter, request *http.Request) {
		realm.ended = append(realm.ended, request.PathValue("id"))
		writer.WriteHeader(http.StatusNoContent)
	})
	admin.HandleFunc("GET /admin/realms/example.com/users/member-1/credentials", func(writer http.ResponseWriter, request *http.Request) {
		credentials := []UserCredential{{Id: "password-1", Type: "password"}}
		if realm.otpSecret != "" {
			credentials = append(credentials, UserCredential{Id: "otp-1", Type: "otp"})
		}
		_ = json.NewEncoder(writer).Encode(credentials)
	})
	admin.HandleFunc("DELETE /admin/realms/example.com/users/member-1/credentials/otp-1", func(writer http.ResponseWriter, request *http.Request) {
		realm.otpSecret = ""
		writer.WriteHeader(http.StatusNoContent)
	})
	admin.HandleFunc("PUT /admin/realms/example.com/mesail/users/member-1/otp", func(writer http.ResponseWriter, request *http.Request) {
		credential := OtpCredential{}
		_ = json.NewDecoder(request.Body).Decode(&credential)
		realm.otpSecret = credential.Secret
		writer.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/admin/", func(writer http.ResponseWriter, request *http.Request) {
		realm.mutex.Lock()
		defer realm.mutex.Unlock()
		if request.Header.Get("Authorization") != "Bearer admin-token" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		admin.ServeHTTP(writer, request)
	})
	realm.server = httptest.NewServer(mux)
	t.Cleanup(realm.server.Close)
	return realm
}

func (realm *accountRealm) manager(t *testing.T) AccountManager {
	t.Helper()
	t.Setenv("TMPDIR", t.TempDir())
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{5}, 32))
	base := realm.server.URL + "/realms/{realm}"
	document := `{"yon":{"oidc":{"issuer":"https://id.mesail.test/realms/{realm}","jwks":"` + base + `/certs","audience":"mesail-api","keycloak":"` + base + `/protocol/openid-connect","admin":"` + realm.server.URL + `/admin/realms/{realm}"},` +
		`"identity":{"client":{"secret":"identity-secret"},"account":{"secret":"account-secret"},"session":{"key":"` + key + `"}}}}`
	manager := AccountManagerFactory{}.FromJson(document)
	manager.Sessions.Client = realm.server.Client()
	return manager
}

func (realm *accountRealm) authorization(t *testing.T) string {
	t.Helper()
	claims := claimsWith("exp", time.Now().Unix()+60)
	return "Bearer " + newTestSigner(t).rsaToken(t, "rsa-1", claims)
}

func (realm *accountRealm) currentCode(t *testing.T) string {
	t.Helper()
	code, err := Totp{}.Code(realm.otpSecret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return code
}

func (realm *accountRealm) withTotp(t *testing.T, manager AccountManager) TotpSetup {
	t.Helper()
	setup, err := manager.SetupTotp(realm.authorization(t), "right-password")
	if err != nil {
		t.Fatal(err)
	}
	code, _ := Totp{}.Code(setup.Secret, time.Now())
	if err := manager.ConfirmTotp(realm.authorization(t), setup.SetupKey, code); err != nil {
		t.Fatal(err)
	}
	return setup
}

func TestTotpEnabledIsFalseWithoutAnOtpCredential(t *testing.T) {
	realm := newAccountRealm(t)
	enabled, err := realm.manager(t).TotpEnabled(realm.authorization(t))
	if err != nil || enabled {
		t.Fatalf("got %v, %v", enabled, err)
	}
}

func TestSetupTotpAnswersTheSecretUriImageAndKey(t *testing.T) {
	realm := newAccountRealm(t)
	setup, err := realm.manager(t).SetupTotp(realm.authorization(t), "right-password")
	if err != nil || setup.Secret == "" || !strings.HasPrefix(setup.Uri, "otpauth://totp/Mesail:ana@example.com?") || !strings.HasPrefix(setup.QrImage, "data:image/png;base64,") || setup.SetupKey == "" {
		t.Fatalf("got %+v, %v", setup, err)
	}
	if realm.signOuts != 1 {
		t.Fatalf("the password check session was not ended: %d", realm.signOuts)
	}
}

func TestSetupTotpRefusesAWrongPassword(t *testing.T) {
	realm := newAccountRealm(t)
	_, err := realm.manager(t).SetupTotp(realm.authorization(t), "wrong")
	if !errors.Is(err, SignInRefused{}) {
		t.Fatalf("got %v", err)
	}
}

func TestConfirmTotpStoresTheCredential(t *testing.T) {
	realm := newAccountRealm(t)
	manager := realm.manager(t)
	setup := realm.withTotp(t, manager)
	enabled, err := manager.TotpEnabled(realm.authorization(t))
	if err != nil || !enabled || realm.otpSecret != setup.Secret {
		t.Fatalf("got %v, %v, stored %q", enabled, err, realm.otpSecret)
	}
}

func TestConfirmTotpRefusesAWrongCode(t *testing.T) {
	realm := newAccountRealm(t)
	manager := realm.manager(t)
	setup, _ := manager.SetupTotp(realm.authorization(t), "right-password")
	err := manager.ConfirmTotp(realm.authorization(t), setup.SetupKey, "000000")
	if !errors.Is(err, InvalidCode{}) || realm.otpSecret != "" {
		t.Fatalf("got %v, stored %q", err, realm.otpSecret)
	}
}

func TestConfirmTotpRefusesAnExpiredSetupKey(t *testing.T) {
	realm := newAccountRealm(t)
	manager := realm.manager(t)
	setup, _ := manager.SetupTotp(realm.authorization(t), "right-password")
	code, _ := Totp{}.Code(setup.Secret, time.Now())
	manager.Now = func() time.Time { return time.Now().Add(601 * time.Second) }
	if err := manager.ConfirmTotp(realm.authorization(t), setup.SetupKey, code); !errors.Is(err, InvalidCode{}) {
		t.Fatalf("got %v", err)
	}
}

func TestConfirmTotpRefusesASetupKeyOfAnotherMember(t *testing.T) {
	realm := newAccountRealm(t)
	manager := realm.manager(t)
	content, _ := json.Marshal(TotpSetupContent{Secret: "JBSWY3DPEHPK3PXP", Subject: "member-2", Domain: "example.com", ExpiresAt: time.Now().Unix() + 600})
	setupKey, _ := manager.Sessions.Sealer.Seal(string(content), "mesail-totp-setup")
	code, _ := Totp{}.Code("JBSWY3DPEHPK3PXP", time.Now())
	if err := manager.ConfirmTotp(realm.authorization(t), setupKey, code); !errors.Is(err, InvalidCode{}) {
		t.Fatalf("got %v", err)
	}
}

func TestConfirmTotpRefusesAForgedSetupKey(t *testing.T) {
	realm := newAccountRealm(t)
	if err := realm.manager(t).ConfirmTotp(realm.authorization(t), "forged", "123456"); !errors.Is(err, InvalidCode{}) {
		t.Fatalf("got %v", err)
	}
}

func TestRemoveTotpDeletesTheCredential(t *testing.T) {
	realm := newAccountRealm(t)
	manager := realm.manager(t)
	realm.withTotp(t, manager)
	err := manager.RemoveTotp(realm.authorization(t), "right-password", realm.currentCode(t))
	if err != nil || realm.otpSecret != "" {
		t.Fatalf("got %v, stored %q", err, realm.otpSecret)
	}
}

func TestRemoveTotpRefusesAWrongCode(t *testing.T) {
	realm := newAccountRealm(t)
	manager := realm.manager(t)
	realm.withTotp(t, manager)
	err := manager.RemoveTotp(realm.authorization(t), "right-password", "000000")
	if !errors.Is(err, SignInRefused{}) || realm.otpSecret == "" {
		t.Fatalf("got %v, stored %q", err, realm.otpSecret)
	}
}

func TestChangePasswordKeepsThisSessionAndEndsTheOthers(t *testing.T) {
	realm := newAccountRealm(t)
	grant, err := realm.manager(t).ChangePassword(realm.authorization(t), "right-password", "a-new-long-password", "")
	if err != nil || grant.SessionKey == "" || grant.Caller.MemberId != "member-1" || realm.password != "a-new-long-password" {
		t.Fatalf("got %+v, %v, password %q", grant, err, realm.password)
	}
	if len(realm.ended) != 1 || realm.ended[0] != "session-b" {
		t.Fatalf("ended %v", realm.ended)
	}
}

func TestChangePasswordNamesTheFailedPolicy(t *testing.T) {
	realm := newAccountRealm(t)
	_, err := realm.manager(t).ChangePassword(realm.authorization(t), "right-password", "short", "")
	refused := AdminRefused{}
	if !errors.As(err, &refused) || refused.Message != "Invalid password: minimum length 12." || refused.Error() == "" || realm.password != "right-password" {
		t.Fatalf("got %v", err)
	}
}

func TestChangePasswordRefusesAWrongCurrentPassword(t *testing.T) {
	realm := newAccountRealm(t)
	_, err := realm.manager(t).ChangePassword(realm.authorization(t), "wrong", "a-new-long-password", "")
	if !errors.Is(err, SignInRefused{}) || realm.password != "right-password" {
		t.Fatalf("got %v", err)
	}
}

func TestAccountRoutesRefuseAForgedCaller(t *testing.T) {
	realm := newAccountRealm(t)
	_, err := realm.manager(t).TotpEnabled("Bearer eyJhbGciOiJub25lIn0.eyJzdWIiOiJhbmEifQ.")
	if !errors.Is(err, SignInRefused{}) {
		t.Fatalf("got %v", err)
	}
}

func TestAccountManagerNeedsItsSettings(t *testing.T) {
	t.Setenv("YON_APPLICATION_JSON", "{}")
	manager := AccountManagerFactory{}.Configured()
	_, enabledErr := manager.TotpEnabled("Bearer x")
	_, setupErr := manager.SetupTotp("Bearer x", "p")
	if !errors.Is(enabledErr, SessionUnavailable{}) || !errors.Is(setupErr, SessionUnavailable{}) || (InvalidCode{}).Error() == "" {
		t.Fatalf("got %v, %v", enabledErr, setupErr)
	}
}
