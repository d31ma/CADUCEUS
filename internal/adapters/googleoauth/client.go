// Package googleoauth implements the confidential Google authorization-code
// flow used by Gmail and Google Workspace IMAP/SMTP accounts.
package googleoauth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/d31ma/caduceus/internal/accounts"
)

const (
	maximumTokenResponse  = 64 << 10
	authorizationEndpoint = "https://accounts.google.com/o/oauth2/v2/auth"
	tokenEndpoint         = "https://oauth2.googleapis.com/token"
	gmailScope            = "https://mail.google.com/"
	calendarEventsScope   = "https://www.googleapis.com/auth/calendar.events"
	calendarListScope     = "https://www.googleapis.com/auth/calendar.calendarlist.readonly"
	contactsScope         = "https://www.googleapis.com/auth/contacts"
)

type Config struct {
	ClientID              string
	ClientSecret          string
	RedirectURI           string
	AuthorizationEndpoint string
	TokenEndpoint         string
	HTTPClient            *http.Client
	Clock                 func() time.Time
}

type Client struct {
	clientID, clientSecret, redirectURI  string
	authorizationEndpoint, tokenEndpoint string
	http                                 *http.Client
	clock                                func() time.Time
}

func New(config Config) (*Client, error) {
	config.ClientID = strings.TrimSpace(config.ClientID)
	config.ClientSecret = strings.TrimSpace(config.ClientSecret)
	redirect, err := url.Parse(strings.TrimSpace(config.RedirectURI))
	if config.ClientID == "" || len(config.ClientID) > 512 || len(config.ClientSecret) < 16 || len(config.ClientSecret) > 8192 || err != nil ||
		!redirect.IsAbs() || redirect.User != nil || redirect.Hostname() == "" || redirect.RawQuery != "" || redirect.Fragment != "" || !secureOrLoopback(redirect) {
		return nil, errors.New("Google OAuth client configuration is invalid")
	}
	if config.AuthorizationEndpoint == "" {
		config.AuthorizationEndpoint = authorizationEndpoint
	}
	if config.TokenEndpoint == "" {
		config.TokenEndpoint = tokenEndpoint
	}
	for _, value := range []string{config.AuthorizationEndpoint, config.TokenEndpoint} {
		parsed, parseErr := url.Parse(value)
		if parseErr != nil || !parsed.IsAbs() || parsed.User != nil || parsed.Hostname() == "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Scheme != "https" {
			return nil, errors.New("Google OAuth provider endpoint is invalid")
		}
	}
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{Timeout: 15 * time.Second}
	}
	if config.Clock == nil {
		config.Clock = time.Now
	}
	return &Client{clientID: config.ClientID, clientSecret: config.ClientSecret, redirectURI: redirect.String(), authorizationEndpoint: config.AuthorizationEndpoint, tokenEndpoint: config.TokenEndpoint, http: config.HTTPClient, clock: config.Clock}, nil
}

func secureOrLoopback(parsed *url.URL) bool {
	if parsed.Scheme == "https" {
		return true
	}
	ip := net.ParseIP(parsed.Hostname())
	return parsed.Scheme == "http" && ip != nil && ip.IsLoopback()
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
	query.Set("redirect_uri", client.redirectURI)
	query.Set("scope", strings.Join([]string{gmailScope, calendarEventsScope, calendarListScope, contactsScope}, " "))
	query.Set("access_type", "offline")
	query.Set("include_granted_scopes", "true")
	query.Set("prompt", "consent")
	query.Set("state", state)
	query.Set("code_challenge", challenge)
	query.Set("code_challenge_method", "S256")
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (client *Client) Exchange(ctx context.Context, code, verifier string) (accounts.Credentials, error) {
	if invalidOpaque(code, 8192) || invalidOpaque(verifier, 1024) {
		return accounts.Credentials{}, accounts.ErrOAuthGrant
	}
	return client.token(ctx, url.Values{
		"client_id": {client.clientID}, "client_secret": {client.clientSecret}, "grant_type": {"authorization_code"},
		"code": {code}, "code_verifier": {verifier}, "redirect_uri": {client.redirectURI},
	}, "")
}

func (client *Client) Refresh(ctx context.Context, refreshToken string) (accounts.Credentials, error) {
	if invalidOpaque(refreshToken, 32768) {
		return accounts.Credentials{}, accounts.ErrOAuthGrant
	}
	return client.token(ctx, url.Values{
		"client_id": {client.clientID}, "client_secret": {client.clientSecret}, "grant_type": {"refresh_token"}, "refresh_token": {refreshToken},
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
	if json.Unmarshal(body, &payload) != nil || !strings.EqualFold(payload.TokenType, "Bearer") || invalidOpaque(payload.AccessToken, 32768) {
		return accounts.Credentials{}, accounts.ErrOAuthGrant
	}
	if payload.RefreshToken == "" {
		payload.RefreshToken = previousRefresh
	}
	if invalidOpaque(payload.RefreshToken, 32768) {
		return accounts.Credentials{}, accounts.ErrOAuthGrant
	}
	seconds, err := expiresIn(payload.ExpiresIn)
	if err != nil {
		return accounts.Credentials{}, accounts.ErrOAuthGrant
	}
	return accounts.Credentials{AccessToken: payload.AccessToken, RefreshToken: payload.RefreshToken, ExpiresAt: client.clock().UTC().Add(time.Duration(seconds) * time.Second)}, nil
}

func expiresIn(raw json.RawMessage) (int64, error) {
	var seconds int64
	if err := json.Unmarshal(raw, &seconds); err != nil {
		var value string
		if json.Unmarshal(raw, &value) != nil {
			return 0, err
		}
		seconds, err = strconv.ParseInt(value, 10, 64)
		if err != nil {
			return 0, err
		}
	}
	if seconds < 60 || seconds > 86400 {
		return 0, errors.New("Google OAuth token lifetime is outside the accepted bound")
	}
	return seconds, nil
}

func invalidOpaque(value string, maximum int) bool {
	return strings.TrimSpace(value) == "" || len(value) > maximum || strings.ContainsAny(value, "\x00\r\n")
}
