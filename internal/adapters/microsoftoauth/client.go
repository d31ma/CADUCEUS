// Package microsoftoauth implements the confidential Microsoft identity
// authorization-code flow used by external Microsoft mailboxes.
package microsoftoauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/d31ma/caduceus/internal/accounts"
)

const maximumTokenResponse = 64 << 10

var tenantName = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9.-]{0,251}[A-Za-z0-9])?$`)

var microsoftScopes = []string{
	"openid", "profile", "email", "offline_access",
	"https://outlook.office.com/IMAP.AccessAsUser.All",
	"https://outlook.office.com/SMTP.Send",
	"Calendars.ReadWrite",
	"Calendars.ReadWrite.Shared",
	"Contacts.ReadWrite",
	"Contacts.ReadWrite.Shared",
}

type Config struct {
	ClientID              string
	ClientSecret          string
	Tenant                string
	RedirectURI           string
	AuthorizationEndpoint string
	TokenEndpoint         string
	HTTPClient            *http.Client
	Clock                 func() time.Time
}

type Client struct {
	clientID              string
	clientSecret          string
	redirectURI           string
	authorizationEndpoint string
	tokenEndpoint         string
	http                  *http.Client
	clock                 func() time.Time
}

func New(config Config) (*Client, error) {
	config.ClientID = strings.TrimSpace(config.ClientID)
	config.ClientSecret = strings.TrimSpace(config.ClientSecret)
	config.Tenant = strings.TrimSpace(config.Tenant)
	if config.Tenant == "" {
		config.Tenant = "organizations"
	}
	if config.ClientID == "" || len(config.ClientID) > 256 || len(config.ClientSecret) < 16 || len(config.ClientSecret) > 8192 ||
		!tenantName.MatchString(config.Tenant) || strings.Contains(config.Tenant, "..") {
		return nil, errors.New("Microsoft OAuth client configuration is invalid")
	}
	if _, err := safeAbsoluteURL(config.RedirectURI, true); err != nil {
		return nil, errors.New("Microsoft OAuth redirect URI is invalid")
	}
	if config.AuthorizationEndpoint == "" {
		config.AuthorizationEndpoint = "https://login.microsoftonline.com/" + url.PathEscape(config.Tenant) + "/oauth2/v2.0/authorize"
	}
	if config.TokenEndpoint == "" {
		config.TokenEndpoint = "https://login.microsoftonline.com/" + url.PathEscape(config.Tenant) + "/oauth2/v2.0/token"
	}
	if _, err := safeAbsoluteURL(config.AuthorizationEndpoint, false); err != nil {
		return nil, errors.New("Microsoft OAuth authorization endpoint is invalid")
	}
	if _, err := safeAbsoluteURL(config.TokenEndpoint, false); err != nil {
		return nil, errors.New("Microsoft OAuth token endpoint is invalid")
	}
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{Timeout: 15 * time.Second}
	}
	if config.Clock == nil {
		config.Clock = time.Now
	}
	return &Client{
		clientID: config.ClientID, clientSecret: config.ClientSecret, redirectURI: config.RedirectURI,
		authorizationEndpoint: config.AuthorizationEndpoint, tokenEndpoint: config.TokenEndpoint,
		http: config.HTTPClient, clock: config.Clock,
	}, nil
}

func (client *Client) AuthorizationURL(state, challenge string) (string, error) {
	if invalidOpaque(state, 1024) || invalidOpaque(challenge, 1024) {
		return "", accounts.ErrOAuthTransaction
	}
	parsed, err := url.Parse(client.authorizationEndpoint)
	if err != nil {
		return "", accounts.ErrOAuthGrant
	}
	query := parsed.Query()
	query.Set("client_id", client.clientID)
	query.Set("response_type", "code")
	query.Set("response_mode", "query")
	query.Set("redirect_uri", client.redirectURI)
	query.Set("scope", strings.Join(microsoftScopes, " "))
	query.Set("state", state)
	query.Set("code_challenge", challenge)
	query.Set("code_challenge_method", "S256")
	query.Set("prompt", "select_account")
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (client *Client) Exchange(ctx context.Context, code, verifier string) (accounts.Credentials, error) {
	if invalidOpaque(code, 8192) || invalidOpaque(verifier, 1024) {
		return accounts.Credentials{}, accounts.ErrOAuthGrant
	}
	return client.token(ctx, url.Values{
		"client_id": {client.clientID}, "client_secret": {client.clientSecret},
		"grant_type": {"authorization_code"}, "code": {code}, "code_verifier": {verifier},
		"redirect_uri": {client.redirectURI}, "scope": {strings.Join(microsoftScopes, " ")},
	}, "")
}

func (client *Client) Refresh(ctx context.Context, refreshToken string) (accounts.Credentials, error) {
	if invalidOpaque(refreshToken, 32768) {
		return accounts.Credentials{}, accounts.ErrOAuthGrant
	}
	return client.token(ctx, url.Values{
		"client_id": {client.clientID}, "client_secret": {client.clientSecret},
		"grant_type": {"refresh_token"}, "refresh_token": {refreshToken},
		"redirect_uri": {client.redirectURI}, "scope": {strings.Join(microsoftScopes, " ")},
	}, refreshToken)
}

type tokenResponse struct {
	AccessToken  string          `json:"access_token"`
	RefreshToken string          `json:"refresh_token"`
	TokenType    string          `json:"token_type"`
	ExpiresIn    json.RawMessage `json:"expires_in"`
}

func (client *Client) token(ctx context.Context, form url.Values, previousRefresh string) (accounts.Credentials, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.tokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return accounts.Credentials{}, accounts.ErrOAuthGrant
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	response, err := client.http.Do(request)
	if err != nil {
		return accounts.Credentials{}, accounts.ErrOAuthGrant
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maximumTokenResponse+1))
	if err != nil || len(body) > maximumTokenResponse || response.StatusCode != http.StatusOK {
		return accounts.Credentials{}, accounts.ErrOAuthGrant
	}
	var payload tokenResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return accounts.Credentials{}, accounts.ErrOAuthGrant
	}
	expires, err := parseExpires(payload.ExpiresIn)
	if err != nil || !strings.EqualFold(payload.TokenType, "Bearer") || invalidOpaque(payload.AccessToken, 32768) {
		return accounts.Credentials{}, accounts.ErrOAuthGrant
	}
	if payload.RefreshToken == "" {
		payload.RefreshToken = previousRefresh
	}
	if invalidOpaque(payload.RefreshToken, 32768) {
		return accounts.Credentials{}, accounts.ErrOAuthGrant
	}
	return accounts.Credentials{AccessToken: payload.AccessToken, RefreshToken: payload.RefreshToken, ExpiresAt: client.clock().UTC().Add(expires)}, nil
}

func parseExpires(raw json.RawMessage) (time.Duration, error) {
	var seconds int64
	if err := json.Unmarshal(raw, &seconds); err != nil {
		var value string
		if stringErr := json.Unmarshal(raw, &value); stringErr != nil {
			return 0, err
		}
		seconds, err = strconv.ParseInt(value, 10, 64)
		if err != nil {
			return 0, err
		}
	}
	if seconds < 60 || seconds > 86_400 {
		return 0, errors.New("OAuth token lifetime is outside the accepted bound")
	}
	return time.Duration(seconds) * time.Second, nil
}

func safeAbsoluteURL(value string, redirect bool) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || !parsed.IsAbs() || parsed.User != nil || parsed.Hostname() == "" || parsed.Fragment != "" || parsed.RawQuery != "" {
		return nil, errors.New("invalid URL")
	}
	if parsed.Scheme != "https" {
		ip := net.ParseIP(parsed.Hostname())
		if parsed.Scheme != "http" || ip == nil || !ip.IsLoopback() {
			return nil, errors.New("URL must use HTTPS")
		}
	}
	if !redirect && parsed.Path == "" {
		return nil, errors.New("provider endpoint path is required")
	}
	return parsed, nil
}

func invalidOpaque(value string, maximum int) bool {
	return strings.TrimSpace(value) == "" || len(value) > maximum || strings.ContainsAny(value, "\x00\r\n")
}

func (client *Client) String() string {
	return fmt.Sprintf("Microsoft OAuth client (%s)", client.authorizationEndpoint)
}
